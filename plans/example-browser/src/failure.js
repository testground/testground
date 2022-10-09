module.exports = async (page, runenv, client) => {
  const userAgent = await page.evaluate(() => {
    return window.navigator.userAgent
  })
  if (userAgent !== 'testground') {
    throw new Error(`failure.js: If you see this message, that means the test failed as expected. User agent is ${userAgent}`)
  }
  runenv.recordMessage('failure.js: If you see this message, something bad has occurred - this test is meant to fail')
}
