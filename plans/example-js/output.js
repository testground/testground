// Demonstrate test output functions
// This method emits two Messages and one Metric (TODO)
module.exports = async (runenv, client) => {
  runenv.recordMessage('Hello, World.')
  runenv.recordMessage(`Additional arguments: ${JSON.stringify(runenv.testInstanceParams)}`)
  // runenv.R().RecordPoint("donkeypower", 3.0)
}
