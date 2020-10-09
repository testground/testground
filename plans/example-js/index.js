const { invokeMap } = require('testground-sdk')

const testcases = {
  failure: require('./failure'),
  output: require('./output'),
  sync: require('./sync'),
  pingpong: require('./pingpong')
}

;(async () => {
  // This is the plan entry point.
  await invokeMap(testcases)
})()
