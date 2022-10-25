const { invokeMap } = require('@testground/sdk')

const testcases = {
  failure: require('./failure'),
  output: require('./output'),
  sync: require('./sync')
}

;(async () => {
  // This is the plan entry point.
  debugger;
  await invokeMap(testcases)
})()
