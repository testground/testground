const { invokeMap } = require('@testground/sdk')

const testcases = {
  helloWorld: require('./helloWorld')
}

;(async () => {
  // This is the plan entry point.
  await invokeMap(testcases)
})()
