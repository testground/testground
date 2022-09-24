const { invokeMap } = require('@testground/sdk')
const createTestCase = require('./playwright-runner')

const testcases = {
  success: createTestCase('./success')
}

;(async () => {
  // This is the plan entry point.
  await invokeMap(testcases)
})()
