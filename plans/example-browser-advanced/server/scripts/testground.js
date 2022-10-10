const { runtime, sync, network } = require('@testground/sdk')

;
(async () => {
  if (!window.testground) {
    window.testground = {}
  }

  let params = window.testground.runtimeParams || {}
  if (!params && window.testground.processEnv) {
    params = runtime.parseRunParams(window.testground.processEnv)
    window.testground.runtimeParams = params
  }
  params.testFromBrowser = true

  const runenv = runtime.newRunEnv(params)
  window.testground.runenv = runenv

  const client = await sync.newBoundClient(runenv)
  window.testground.client = client

  window.testground.network = network.newClient(client, runenv)

  window.testground.runenv.recordMessage('testground initialized')
})()
