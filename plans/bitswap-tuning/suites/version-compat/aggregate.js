'use strict'

const { parseMetrics, groupBy } = require('../common')
const fs = require('fs')

const [,, basePath] = process.argv
if (!basePath) {
  throw new Error("usage: node aggregate.js <basepath>")
}

run()

function run () {
  const data = fs.readFileSync(0).toString() // STDIN_FILENO = 0
  const metrics = parseMetrics(data)

  const byGroupName = groupBy(metrics, 'groupName')
  for (const [groupName, metrics] of Object.entries(byGroupName)) {
    const byNodeType = groupBy(metrics, 'nodeType')
    for (const [nodeType, metrics] of Object.entries(byNodeType)) {
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
        writeResults(groupName, nodeType, name, 'average', points)
      }
    }
  }
}

function writeResults (groupName, nodeType, name, statName, points) {
  const filePath = basePath + `.${groupName}.${nodeType}.${name}.${statName}.csv`
  const content = points.map(p => p.join(',')).join('\n') + '\n'
  fs.writeFileSync(filePath, content)
}
