module.exports = async (page, runenv, client) => {
  runenv.recordStart()

  const browserPlatform = await page.evaluate(() => {
    return window.navigator.userAgentData.platform
  })
  if (!/linux/i.exec(browserPlatform)) {
    throw new Error(`unexpected browser platform: ${browserPlatform}`)
  }

  runenv.recordMessage('playwright: success: linux platform detected')
  runenv.recordSuccess()
}
