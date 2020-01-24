'use strict'

const fs = require('fs')

const [,, basePath] = process.argv
if (!basePath) {
  throw new Error("usage: node aggregate.js <basepath>")
}

run()

function run () {
  const data = fs.readFileSync(0).toString() // STDIN_FILENO = 0
  const metrics = parseMetrics(data)

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
      writeResults(nodeType, name, 'average', points)
    }
  }
}

function writeResults (nodeType, name, statName, points) {
  const filePath = basePath + `.${nodeType}.${name}.${statName}.csv`
  const content = points.map(p => p.join(',')).join('\n') + '\n'
  fs.writeFileSync(filePath, content)
}

function parseMetrics (data) {
  const metrics = []
  for (const line of data.toString().split('\n')) {
    let log = {}
    try {
      log = JSON.parse(line)
    } catch (e) {
    }

    if (log.eventType === 'Metric') {
      const metric = log.metric
      const parts = metric.name.split(/\//)
      // [ 'run:1', 'seq:2', 'file-size:10485760', 'Seed', 'msgs_rcvd' ]
      if (parts.length !== 5) {
        throw new Error(`Unexpected metric format ${metric}`)
      }

      const [runDef, seqDef, fileSizeDef, nodeType, metricName] = parts
      const run = runDef.split(':')[1]
      const seq = seqDef.split(':')[1]
      const fileSize = fileSizeDef.split(':')[1]

      metrics.push({ run, seq, fileSize, nodeType, name: metricName, value: metric.value })
    }
  }

  return metrics
}

function groupBy (arr, key) {
  const res = {}
  for (const i of arr) {
    const val = i[key]
    res[val] = res[val] || []
    res[val].push(i)
  }
  return res
}
