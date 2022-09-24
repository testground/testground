const { chromium, firefox } = require('playwright')

module.exports = (module) => {
  const testFn = require(module)
  return (runenv, client) => {
    runTestFn(runenv, client, testFn)
  }
}

async function runTestFn (runenv, client, fn) {
  let browser

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

  return await fn(page, runenv, client)
}
