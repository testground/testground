'use strict'

// node chart.js \
//  -d ./results/bw1024MB-3x3 \
//  -m time_to_fetch \
//  -b 1024 \
//  -l 5 \
//  -xlabel 'File size (MB)' \
//  -ylabel 'Time to fetch (s)' \
//  -xscale '9.53674316e-7' \
//  -yscale '1e-9' \
//  && gnuplot ./results/bw1024MB-3x3/time_to_fetch.plot > ./results/bw1024MB-3x3/bw1024MB-3x3.svg

const { parseArgs, getLineColor } = require('../common')

const fs = require('fs')
const path = require('path')

const args = parseArgs(['d', 'm'], {
  'l': 5, // latency in ms
  'b': 1024, // bandwidth in MB
  'xscale': 1,
  'yscale': 1
})

function getPlotfile () {
  let plotFile = `
    # Output W3C Scalable Vector Graphics
    set terminal svg

    # Read comma-delimited data from file
    set datafile separator comma

    # Put the line labels at the top left
    set key left
  `

  if (args.xlabel) {
    plotFile += `set xlabel '${args.xlabel}'\n`
  }

  if (args.ylabel) {
    plotFile += `set ylabel '${args.ylabel}'\n`
  }

  return plotFile
}

function run (metricName, latencyMS, bandwidthMB) {
  let outFiles = parseOutputFiles()
  outFiles = outFiles.filter((file) => file.name === metricName)
  outputPlot (metricName, outFiles, latencyMS, bandwidthMB)
}

// Files are in a directory with subdirs for each branch, eg
// results/master/...
// results/mybranch/...
function parseOutputFiles () {
  const res = []
  const dir = args.d
  const subdirs = fs.readdirSync(dir)
  for (const subdir of subdirs) {
    const subdirPath = path.join(dir, subdir)
    if (fs.lstatSync(subdirPath).isDirectory()) {
      const files = fs.readdirSync(subdirPath)
      for (const file of files) {
        const matches = file.match(/(([0-9])+sx([0-9])+l)\.Leech\.(.+)\.average\.csv/)
        if (matches) {
          const [, label, seeds, leeches, name] = matches
          const filePath = path.join(subdirPath, file)

          try {
            fs.accessSync(filePath)
            res.push({ branch: subdir, name, seeds, leeches, filePath })
          } catch (e) {
          }
        }
      }
    }
  }
  return res
}

function outputPlot (metricName, outFiles, latencyMS, bandwidthMB) {
  const scaledCols = `(column(1)*(${args.xscale})):(column(2)*(${args.yscale}))`

  let output = getPlotfile()
  output += `set title 'Bitswap (${latencyMS}ms latency, ${bandwidthMB}MB bandwidth)'\n`

  const plotArgs = []
  for (const [i, { branch, seeds, leeches, filePath }] of outFiles.entries()) {
    const id = `${branch}${seeds}x${leeches}`

    const showLine = true //branch === 'master' && seeds > 1
    const fn = `f${id}(x)`

    if (showLine) {
      output += `${fn} = a${id}*x + b${id}\n`
      output += `fit ${fn} '${filePath}' using ${scaledCols} via a${id},b${id}\n`
      // output += `${fn} = a${id}*x**2 + b${id}*x + c${id}\n`
      // output += `fit ${fn} '${filePath}' using 1:2 via a${id},b${id},c${id}\n`
    }

    const lineColor = getLineColor(branch, seeds, leeches)
    const title = `${branch}: ${seeds} seeds / ${leeches} leeches`
    plotArgs.push(`'${filePath}' using ${scaledCols} with points title "" lc rgb '${lineColor}' pointtype 3 pointsize 0.5`)
    // plotArgs.push(`'${filePath}' smooth csplines title "${title}" lc rgb '${lineColor}' linewidth 2`)
    if (showLine) {
      plotArgs.push(`${fn} title "${title}" lc rgb '${lineColor}' linewidth 4`)
    }
  }
  const plotCmd = 'plot ' + plotArgs.join(',\\\n  ')
  output += plotCmd + '\n'

  const filePath = path.join(args.d, `${metricName}.plot`)
  fs.writeFileSync(filePath, output)
}

function usage () {
  console.log("usage: node graph.js -d <directory> -m <metric> -l <latencyMS> -b <bandwidthMB>")
}

run(args.m, args.l, args.b)
