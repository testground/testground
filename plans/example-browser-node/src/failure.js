module.exports = async (runenv, client) => {
  runenv.recordMessage('This is what happens when there is a failure')
  throw new Error('intentional oops')
}
