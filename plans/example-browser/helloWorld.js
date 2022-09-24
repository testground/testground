const { chromium, firefox } = require('playwright')
const { expect } = require('@playwright/test')

module.exports = async (runenv, client) => {
  let browser

  runenv.recordStart()

  runenv.recordMessage('playwright: launching browser and opening new page')
  if (/firefox/i.exec(runenv.testInstanceParams.browser)) {
    browser = await firefox.launch()
    runenv.recordMessage('playwright: firefox launched')
  } else {
    browser = await chromium.launch()
    runenv.recordMessage('playwright: chromium launched')
  }
  const page = await browser.newPage()
  runenv.recordMessage('playwright: new page opened')

  await expect(page.evaluate(() => {
    return window.navigator.userAgentData.platform
  })).toHaveText('Linux')

  runenv.recordMessage('playwright: success: linux platform detected')
  runenv.recordSuccess()
}
