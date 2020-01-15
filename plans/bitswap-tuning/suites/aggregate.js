'use strict'

const fs = require('fs')

const [,, filepath] = process.argv
if (!filepath) {
  throw new Error("usage: node parse.js <filepath>")
}

const data = fs.readFileSync(filepath)
const logs = []
for (const line of data.toString().split('\n')) {
  try {
    const log = JSON.parse(line)
    logs.push(log)
  } catch (e) {
    // Ignore lines that cannot be parsed as JSON
  }
}

const metricDefs = new Map()
const instances = new Map()
for (const log of logs) {
  let instance = instances.get(log.instanceId)
  if (!instance) {
    instance = { metrics: [] }
    instances.set(log.instanceId, instance)
  }

  // Figure out the node type for each instance
  if (log.msg === 'I am a seed') {
    instance.isSeed = true
  } else if (log.msg === 'I am a leech') {
    instance.isSeed = false
  }

  // Collect metrics for each instance
  if (log.eventType === 'Metric' && log.metric) {
    const metric = log.metric
    instance.metrics.push(metric)
    if (!metricDefs.has(metric.name)) {
      metricDefs.set(metric.name, { unit: metric.unit })
    }
  }
}

const countWithIsSeed = (val) => [...instances.values()].reduce((a, i) => {
  return a + (i.isSeed == val ? 1 : 0)
}, 0)

const stats = {
  seed: {
    count: countWithIsSeed(true),
    metrics: new Map()
  },
  leech: {
    count: countWithIsSeed(false),
    metrics: new Map()
  }, passive: {
    count: countWithIsSeed(null),
    metrics: new Map()
  }
}

// Collect metrics by node type, grouped by metric name
for (const instance of instances.values()) {
  for (const metric of instance.metrics) {
    const statsMetrics = (() => {
      switch(instance.isSeed) {
        case true: return stats.seed.metrics
        case false: return stats.leech.metrics
        default: return stats.passive.metrics
      }
    })()
    let metrics = statsMetrics.get(metric.name)
    if (!metrics) {
      metrics = []
      statsMetrics.set(metric.name, metrics)
    }
    metrics.push(metric.value)
  }
}

// Aggregate statistics for each metric
for (const [statType, stat] of Object.entries(stats)) {
  const asObject = {}
  for (const [name, metrics] of stat.metrics) {
    const metrics = stat.metrics.get(name)
    asObject[name] = {
      ...metricDefs.get(name),
      stats: {
        min: Math.min(...metrics),
        max: Math.max(...metrics),
        average: metrics.reduce((a,b) => a + b, 0) / metrics.length,
        mean: metrics.sort()[Math.floor(metrics.length / 2)]
      }
    }
  }
  stat.metrics = asObject
}

console.log(JSON.stringify(stats, null, 2))
