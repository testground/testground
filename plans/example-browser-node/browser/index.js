const { chromium } = require('playwright')

const spawnServer = require('../server')

function sleep (ms) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

;(async () => {
  spawnServer(8080)

  let browser
  try {
    browser = await chromium.launch({
      args: [
        '--remote-debugging-address=0.0.0.0',
        `--remote-debugging-port=${process.env.DEBUG_PORT || 9222}`
      ]
    })

    const page = await browser.newPage()

    page.on('console', (message) => {
      console.log(`[${message.type()}] ${message.locaton()}: ${message.text()} — ${message.args()}`)
    })

    await page.goto('http://127.0.0.1:8080')

    // TODO: wait for the test to finish somehow :|
    await sleep(60000)
  } finally {
    if (browser) {
      try {
        await browser.close()
      } catch (_) {}
    }
  }
})()
