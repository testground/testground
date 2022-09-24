module.exports = async (page, runenv, client) => {
  const userAgent = await page.evaluate(() => {
    return window.navigator.userAgent
  })
  if (userAgent !== 'testground') {
    throw new Error(`expected user agent 'testground', unexpected: ${userAgent}`)
  }

  // we do not ever expect to reach here, this is a failure test
  runenv.recordMessage('playwright: success: testground platform detected')
}
