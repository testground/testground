'use strict'

// node ./plans/bitswap-tuning/suites/chart.js \
//  -d ./results/bw1024MB-3x3 \
//  -m time_to_fetch \
//  --b=1024 \
//  -xlabel 'File size (MB)' \
//  -ylabel 'Time to fetch (s)' \
//  -xscale '9.53674316e-7' \
//  -yscale '1e-9' \
//  && gnuplot ./results/bw1024MB-3x3/time_to_fetch.plot > ./results/bw1024MB-3x3/bw1024MB-3x3.svg

const fs = require('fs')
const path = require('path')

const args = parseArgs(['d', 'm'], {
  'l': 5,
  'b': 1024,
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
  let outFiles = parseOutputFiles(metricName)
  outFiles = outFiles.filter((file) => file.latencyMS == latencyMS && file.bandwidthMB == bandwidthMB)
  outputPlot (metricName, outFiles, latencyMS, bandwidthMB)
}

// Files are in a directory with subdirs for each branch, eg
// results/master/...
// results/mybranch/...
function parseOutputFiles (metricName) {
  const res = []
  const dir = args.d
  const subdirs = fs.readdirSync(dir)
  for (const subdir of subdirs) {
    const subdirPath = path.join(dir, subdir)
    if (fs.lstatSync(subdirPath).isDirectory()) {
      const files = fs.readdirSync(subdirPath)
      for (const file of files) {
        const matches = file.match(/(([0-9])+sx([0-9])+l)-([0-9]+)ms-bw([0-9]+)\.(.+)\.raw/)
        if (matches) {
          const [, label, seeds, leeches, latencyMS, bandwidthMB, name] = matches
          const filePath = path.join(subdirPath, `${label}.Leech.${metricName}.average.csv`)

          try {
            fs.accessSync(filePath)
            res.push({ branch: subdir, seeds, leeches, latencyMS, bandwidthMB, filePath })
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

const lineColors = [
  ['#bbe1fa', '#3282b8', '#0f4c75', '#1b262c'],
  ['#f1bc31', '#e25822', '#b22222', '#7c0a02'],
  ['#64e291', '#a0cc78', '#589167', '#207561'],
]
const usedColors = []
function getLineColor (branch, seeds, leeches) {
  const branchIndex = getBranchIndex(branch, seeds, leeches)
  const branchColors = lineColors[branchIndex % lineColors.length]
  const colorIndex = usedColors[branchIndex].count % branchColors.length
  usedColors[branchIndex].count++
  return branchColors[colorIndex]
}

function getBranchIndex (branch, seeds, leeches) {
  for (const [i, b] of Object.entries(usedColors)) {
    if (b.name === branch) {
      return i
    }
  }
  usedColors.push({ name: branch, count: 0 })
  return usedColors.length - 1
}

function parseArgs (required, defaults) {
  const res = {}
  const args = process.argv.slice(2)
  for (let i = 0; i < args.length; i++) {
    const arg = args[i]
    if (arg[0] == '-') {
      let k, v
      if (arg[1] == '-') {
        [k, v] = arg.substring(2).split('=')
      } else {
        k = arg.substring(1)
        v = args[i + 1]
      }
      if (!k || v == null) {
        throw new Error(usage())
      }
      res[k] = v
    }
  }

  for (const [k, v] of Object.entries(defaults)) {
    if (res[k] == null) {
      res[k] = v
    }
  }

  for (const req of required) {
    if (res[req] == null) {
      throw new Error(usage())
    }
  }

  return res
}

function usage () {
  console.log("usage: node graph.js -d <directory> -m <metric> -l <latencyMS> -b <bandwidthMB>")
}

run(args.m, args.l, args.b)
