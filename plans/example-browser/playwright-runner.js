const { chromium, firefox } = require('playwright')

module.exports = (module) => {
  const testFn = require(module)
  return async (runenv, client) => {
    runenv.recordStart()
    await runTestFn(runenv, client, testFn)
    runenv.recordSuccess()
  }
}

async function runTestFn (runenv, client, fn) {
  let browser
  let result

  try {
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

    result = await fn(page, runenv, client)
  } finally {
    if (browser) {
      try {
        await browser.close()
        runenv.recordMessage('playwright: browser closed')
      } catch (error) {
        runenv.recordMessage(`playwright: failed to close browser: ${error}`)
      }
    }
  }

  return result
}
