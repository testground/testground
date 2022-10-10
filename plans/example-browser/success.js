module.exports = async (page, runenv, client) => {
  const userAgent = await page.evaluate(() => {
    return window.navigator.userAgent
  })
  if (!/linux/i.exec(userAgent)) {
    throw new Error(`unexpected linux user agent: ${userAgent}`)
  }

  runenv.recordMessage('playwright: success: linux platform detected')
}
