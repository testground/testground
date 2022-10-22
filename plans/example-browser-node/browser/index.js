const { chromium } = require('playwright')
const { exit } = require('process')

const spawnServer = require('../server')

function sleep (ms) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

;(async () => {
  spawnServer(8080)

  let browser
  try {
    const chomeDebugPort = process.env.DEBUG_PORT || 9222
    console.log(`launching chromium browser with exposed debug port: ${chomeDebugPort}`)
    browser = await chromium.launch({
      args: [
        '--remote-debugging-address=0.0.0.0',
        `--remote-debugging-port=${chomeDebugPort}`
      ]
    })

    const page = await browser.newPage()

    page.on('console', (message) => {
      console.log(`[${message.type()}] ${message.locaton()}: ${message.text()} — ${message.args()}`)
    })

    console.log('opening up testplan webpage on localhost')
    await page.goto('http://127.0.0.1:8080')

    // TODO: wait for the test to finish somehow :|
    console.log('waiting for 60s until exiting... (TODO fix this)')
    await sleep(60000)
  } finally {
    if (browser) {
      try {
        await browser.close()
      } catch (_) {}
    }
    exit(0)
  }
})()
