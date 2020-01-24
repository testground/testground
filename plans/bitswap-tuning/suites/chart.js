'use strict'

const fs = require('fs')
const path = require('path')

const [,, dir, metric] = process.argv
if (!dir || !metric) {
  throw new Error("usage: node graph.js <directory> <metric>")
}

const plotFile = `
  # Output W3C Scalable Vector Graphics
  set terminal svg

  # Read comma-delimited data from file
  set datafile separator comma

  # Set graph title
  set title 'Bitswap (5ms latency, 1Gib bandwidth)'

  # Put the line labels at the top left
  set key left

  # Set label of x-axis
  set xlabel 'File size (MB)'

  # Set label of y-axis
  set ylabel 'Time to fetch (s)'
`

function parseOutputFiles (metricName) {
  const res = []
  const subdirs = fs.readdirSync(dir)
  for (const subdir of subdirs) {
    const subdirPath = path.join(dir, subdir)
    if (fs.lstatSync(subdirPath).isDirectory()) {
      const files = fs.readdirSync(subdirPath)
      for (const file of files) {
        const matches = file.match(/(([0-9])+sx([0-9])+l)-5ms-bw1024\.(.+)\.raw/)
        if (matches) {
          const [, label, seeds, leeches, name] = matches
          const filePath = path.join(subdirPath, `${label}.Leech.${metricName}.average.csv`)

          try {
            fs.accessSync(filePath)
            res.push({ branch: subdir, seeds, leeches, filePath })
          } catch (e) {
          }
        }
      }
    }
  }
  return res
}

function run (metricName) {
  const outFiles = parseOutputFiles(metricName)
  outputPlot (metricName, outFiles)
}

function outputPlot (metricName, outFiles) {
  const scaledCols = '(column(1)/(1024*1024)):(column(2)/(1e9))'

  let output = plotFile
  const plotArgs = []
  for (const [i, { branch, seeds, leeches, filePath }] of outFiles.entries()) {
    const id = `${branch}${seeds}x${leeches}`
    const fn = `f${id}(x)`
    output += `${fn} = a${id}*x + b${id}\n`
    output += `fit ${fn} '${filePath}' using ${scaledCols} via a${id},b${id}\n`
    // output += `${fn} = a${id}*x**2 + b${id}*x + c${id}\n`
    // output += `fit ${fn} '${filePath}' using 1:2 via a${id},b${id},c${id}\n`

    const title = `${branch}: ${seeds} seeds / ${leeches} leeches`
    plotArgs.push(`'${filePath}' using ${scaledCols} with points title "" lc ${i+1} pointtype 3 pointsize 0.5`)
    // plotArgs.push(`'${filePath}' smooth csplines title "${title}" lc ${i+1} linewidth 2`)
    plotArgs.push(`${fn} title "${title}" lc ${i+1} linewidth 2`)
  }
  const plotCmd = 'plot ' + plotArgs.join(',\\\n  ')
  output += plotCmd + '\n'

  const filePath = path.join(dir, `${metricName}.plot`)
  fs.writeFileSync(filePath, output)
}

run(metric)
