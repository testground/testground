// Demonstrate test output functions
// This method emits two Messages and one Metric
module.exports = async (runenv, client) => {
  runenv.recordMessage('Hello, World.')
  runenv.recordMessage(`Additional arguments: ${JSON.stringify(runenv.testInstanceParams)}`)
}
