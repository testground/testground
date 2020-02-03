'use strict'

// node chart.js \
//  -d ./results/bw1024MB-3x3 \
//  -branch master \
//  -m blks_sent \
//  -b 1024 \
//  -xlabel 'File size (MB)' \
//  -ylabel 'Blocks Sent' \
//  -xscale '9.53674316e-7' \
//  && gnuplot ./results/bw1024MB-3x3/master.blks_sent.plot > ./results/bw1024MB-3x3/bw1024MB-3x3.svg

const { parseArgs, getLineColor } = require('../common')
const fs = require('fs')
const path = require('path')

const args = parseArgs(['d', 'm'], {
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

function run (metricName, bandwidthMB, branch) {
  let outFiles = parseOutputFiles()
  outFiles = outFiles.filter((file) => file.name === metricName)
  if (branch) {
    outFiles = outFiles.filter((file) => file.branch === branch)
  }
  outputPlot (metricName, outFiles, bandwidthMB, branch)
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
        const matches = file.match(/(([0-9])+sx([0-9])+l)\.Seed\.([0-9]+)\.(.+)\.average\.csv/)
        if (matches) {
          const [, label, seeds, leeches, latencyMS, name] = matches
          const filePath = path.join(subdirPath, file)

          try {
            fs.accessSync(filePath)
            res.push({ branch: subdir, name, seeds, leeches, latencyMS, filePath })
          } catch (e) {
          }
        }
      }
    }
  }
  return res
}

function outputPlot (metricName, outFiles, bandwidthMB, branch) {
  const scaledCols = `(column(1)*(${args.xscale})):(column(2)*(${args.yscale}))`

  let output = getPlotfile()
  output += `set title 'Bitswap (${bandwidthMB}MB bandwidth)'\n`

  const plotArgs = []
  for (const [i, { branch, seeds, leeches, latencyMS, filePath }] of outFiles.entries()) {
    const id = `${branch}${seeds}x${leeches}x${latencyMS}`

    const showLine = true //branch === 'master' && seeds > 1
    const fn = `f${id}(x)`

    if (showLine) {
      output += `${fn} = a${id}*x + b${id}\n`
      output += `fit ${fn} '${filePath}' using ${scaledCols} via a${id},b${id}\n`
      // output += `${fn} = a${id}*x**2 + b${id}*x + c${id}\n`
      // output += `fit ${fn} '${filePath}' using 1:2 via a${id},b${id},c${id}\n`
    }

    const lineColor = getLineColor(branch)
    const title = `${branch}: ${latencyMS}ms latency`
    plotArgs.push(`'${filePath}' using ${scaledCols} with points title "" lc rgb '${lineColor}' pointtype 3 pointsize 0.5`)
    // plotArgs.push(`'${filePath}' smooth csplines title "${title}" lc rgb '${lineColor}' linewidth 2`)
    if (showLine) {
      plotArgs.push(`${fn} title "${title}" lc rgb '${lineColor}' linewidth 4`)
    }
  }
  const plotCmd = 'plot ' + plotArgs.join(',\\\n  ')
  output += plotCmd + '\n'

  let fileName = `${metricName}.plot`
  if (branch) {
    fileName = `${branch}.${fileName}`
  }
  const filePath = path.join(args.d, fileName)
  fs.writeFileSync(filePath, output)
}

function usage () {
  console.log("usage: node graph.js -d <directory> -m <metric> -branch <branch> -b <bandwidthMB>")
}

run(args.m, args.b, args.branch)
