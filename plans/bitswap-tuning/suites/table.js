'use strict'

const fs = require('fs')

const padding = 3
run()

function run () {
  let data = JSON.parse(fs.readFileSync(0).toString()) // STDIN_FILENO = 0

  data = preprocess(data, (metricName, statName, statValue) => {
    if (metricName === 'time_to_fetch') {
      const inSeconds = statValue / 1e9
      return Math.round(inSeconds * 100) / 100 + 's'
    }
    return statValue
  })
  console.log(tableToString(createTable(data)))
}

// Convert JSON input into tabular output
function createTable (data) {
  // Filter out node types (seed/leech/passive) with 0 entries
  const entries = Object.entries(data).filter(([, s]) => s.count > 0)

  const table = []
  for (const [name, stats] of entries) {
    // Create a header row with the name of the node type (seed/leech/passive)
    // and the name of each stat (max/min/etc)
    const header = []
    const metrics = stats.metrics
    header.push(name + ' (' + stats.count + ')')

    const statNames = Object.keys(stats.metrics[objFirstKey(stats.metrics)].stats).sort()
    for (const statName of statNames) {
      header.push(statName)
    }
    table.push(header)

    // Create a row for each metric (eg data_rcvd) with the value
    // for each stat
    for (const metricName of Object.keys(stats.metrics).sort()) {
      const row = [metricName]
      for (const statName of statNames) {
        const value = stats.metrics[metricName].stats[statName]
        row.push(value)
      }
      table.push(row)
    }

    // Create a blank line
    table.push([])
  }
  return table
}

function tableToString (table) {
  const colWidths = []
  for (const [r, row] of table.entries()) {
    for (const [c, col] of row.entries()) {
      const valWidth = (col + "").length
      if (!colWidths[c] || colWidths[c] < valWidth) {
        colWidths[c] = valWidth
      }
    }
  }

  const lines = []
  for (const [r, row] of table.entries()) {
    let line = ""
    for (const [c, col] of row.entries()) {
      const valWidth = (col + "").length
      const spaces = ' '.repeat(padding + colWidths[c] - valWidth)

      // Left justify first column, right justify other columns
      if (c === 0) {
        line += col + spaces
      } else {
        line += spaces + col
      }
    }
    lines.push(line)
  }

  return lines.join('\n')
}

function objFirstKey (m) {
  return Object.keys(m)[0]
}

function preprocess (data, preprocessStat) {
  for (const stats of Object.values(data)) {
    for (const [metricName, metricStats] of Object.entries(stats.metrics)) {
      for (const [k, v] of Object.entries(metricStats.stats)) {
        metricStats.stats[k] = preprocessStat(metricName, k, v)
      }
    }
  }
  return data
}
