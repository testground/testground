module.exports = async (runenv) => {
  runenv.recordMessage('This is what happens when there is a failure')
  throw new Error('intentional oops')
}