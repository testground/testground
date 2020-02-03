'use strict'

const { parseMetrics, groupBy } = require('../common')
const fs = require('fs')

const [,, basePath] = process.argv
if (!basePath) {
  throw new Error("usage: node peer-choice-process.js <basepath>")
}

run()

function run () {
  const data = fs.readFileSync(0).toString() // STDIN_FILENO = 0
  const metrics = parseMetrics(data)

  const byNodeType = groupBy(metrics, 'nodeType')
  for (const [nodeType, metrics] of Object.entries(byNodeType)) {
    const byLatency = groupBy(metrics, 'latencyMS')
    for (const [latencyMS, metrics] of Object.entries(byLatency)) {
      const byName = groupBy(metrics, 'name')
      for (const [name, metrics] of Object.entries(byName)) {
        const byRun = groupBy(metrics, 'run')
        const points = []
        for (const runNum of Object.keys(byRun).sort(i => parseInt(i))) {
          const metrics = byRun[runNum]
          const byFileSize = groupBy(metrics, 'fileSize')
          for (const [fileSize, metrics] of Object.entries(byFileSize)) {
            const average = metrics.reduce((a, m) => a + m.value, 0) / metrics.length
            points.push([fileSize, average])
          }
        }
        writeResults(nodeType, latencyMS, name, 'average', points)
      }
    }
  }
}

function writeResults (nodeType, latencyMS, name, statName, points) {
  const filePath = basePath + `.${nodeType}.${latencyMS}.${name}.${statName}.csv`
  const content = points.map(p => p.join(',')).join('\n') + '\n'
  fs.writeFileSync(filePath, content)
}
