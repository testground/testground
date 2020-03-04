// node gentomls.js -def ./two-thirds-two-seeds-def.js -o /tmp/gentoml --latency_ms=100 --seed_fraction=2/3

const { groupBy, parseArgs } = require('../common')

const fs = require('fs')

const globalTestParams = {
  latency_ms: 5,
  bandwidth_mb: 1024,
  file_size: "1485760",
  seed_fraction: "",
  run_count: "1"
}

const args = parseArgs(['o', 'def'])
const basePath = args.o
const def = require(process.cwd() + '/' + args.def)

for (k of Object.keys(globalTestParams)) {
  if (args[k]) {
    globalTestParams[k] = args[k]
  }
}

const header = `
[metadata]
name    = "transfer-versions"
author  = "dirkmc"

[global]
plan    = "bitswap-tuning"
case    = "transfer"
builder = "docker:go"
runner  = "local:docker"
`

function runSuite (name, suite) {
  const versions = groupBy(def.versions, "name")

  let output = `# ${name}\n`
  output += header

  const instanceCount = [...suite.seed, ...suite.leech].reduce((a, s) => a + s.count, 0)
  output += `total_instances = ${instanceCount}\n`

  let seedCount = 0
  let leechCount = 0
  const groups = {}
  for (const seed of suite.seed) {
    groups[seed.version] = groups[seed.version] || { seeds: 0, leeches: 0 }
    groups[seed.version].seeds += seed.count
    seedCount += seed.count
  }
  for (const leech of suite.leech) {
    groups[leech.version] = groups[leech.version] || { seeds: 0, leeches: 0 }
    groups[leech.version].leeches += leech.count
    leechCount += leech.count
  }

  for (const [version, group] of Object.entries(groups)) {
    versionDef = versions[version][0]

    let params = {
      seed_count: seedCount,
      leech_count: leechCount
    }
    if (Object.keys(groups).length > 1) {
      params[versionDef.testgroundId + '_leech_count'] = group.leeches
    }
    params = {...globalTestParams, ...params}

    const paramOut = "{" + Object.entries(params).map(([k, v]) => `${k} = "${v}"`).join(', ') + "}"

    output += `\n[[groups]]\n`
    output += `id = "${versionDef.testgroundId}"\n`
    output += `instances = { count = ${group.seeds + group.leeches} }\n`
    output += `\n`
    output += `  [groups.build]\n`
    output += `  # ref for ${versionDef.name}\n`
    output += `  dependencies = [{ module = "github.com/ipfs/go-bitswap", version = "${versionDef.ref}"} ]\n`
    output += `\n`
    output += `  [groups.run]\n`
    output += `  test_params = ${paramOut}\n`
  }
  writeFile(name, output)
}

function writeFile (name, output) {
  const fsName = name.replace(/[^a-zA-Z0-9-]/g, '_')
  const filePath = basePath + `/${fsName}.toml`
  fs.writeFileSync(filePath, output)
}

function run () {
  for (const [name, suite] of Object.entries(def.suites)) {
    runSuite(name, suite)
  }
}

run()
