'use strict'

function parseMetrics (data) {
  const metrics = []
  for (const line of data.toString().split('\n')) {
    let log = {}
    try {
      log = JSON.parse(line)
    } catch (e) {
    }

    if ((log.event || {}).type === 'metric') {
      const metric = log.event.metric
      const res = {}
      const parts = metric.name.split(/\//)
      for (const part of parts) {
        const [k, v] = part.split(':')
        res[k] = v
      }
      res.value = metric.value

      metrics.push(res)
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

function parseArgs (required = [], defaults = {}) {
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

const lineColors = [
  ['#bbe1fa', '#3282b8', '#0f4c75', '#1b262c'],
  ['#f1bc31', '#e25822', '#b22222', '#7c0a02'],
  ['#64e291', '#a0cc78', '#589167', '#207561'],
  ['#cc6666', '#aa4444', '#992222', '#660000'],
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

module.exports = {
  parseMetrics,
  groupBy,
  parseArgs,
  getLineColor
}
