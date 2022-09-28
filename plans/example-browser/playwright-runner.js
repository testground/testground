const { chromium, firefox } = require('playwright')

// create a testground testCase function
// which will open a page in a desired browser
// and run the default exported function from the given
// module against that page.
module.exports = (module) => {
  const testFn = require(module)
  return async (runenv, client) => {
    runenv.recordStart()
    await runTestFn(runenv, client, testFn)
    runenv.recordSuccess()
  }
}

// utility function to launch the desired browser,
// open a new (blank) page on it and run the given test against it.
async function runTestFn (runenv, client, fn) {
  let browser
  let result

  try {
    runenv.recordMessage('playwright: launching browser and opening new page')

    switch (runenv.testInstanceParams.browser || 'chromium') {
      case 'firefox':
        browser = await firefox.launch()
        runenv.recordMessage('playwright: firefox launched')
        break

      case 'chromium':
        browser = await chromium.launch()
        runenv.recordMessage('playwright: chromium launched')
        break

      default:
        throw new Error(`invalid browser test parameter: ${runenv.testInstanceParams.browser}`)
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
