const { chromium } = require('playwright')
const { exit } = require('process')

const spawnServer = require('../server')

;(async () => {
  spawnServer(8080)

  let browser
  try {
    const chromeDebugPort = process.env.CHROME_DEBUG_PORT || 9222
    console.log(`launching chromium browser with exposed debug port: ${chromeDebugPort}`)
    browser = await chromium.launch({
      args: [
        '--remote-debugging-address=0.0.0.0',
        `--remote-debugging-port=${chromeDebugPort}`
      ]
    })

    const page = await browser.newPage()

    page.on('console', (message) => {
      const loc = message.location();
      console.log(`[${message.type()}] ${loc.url}@L${loc.lineNumber}:C${loc.columnNumber}: ${message.text()} — ${message.args()}`)
    })

    console.log('prepare page window (global) environment')
    await page.addInitScript((nodeEnv) => {
      const env = {}
      for (const k in nodeEnv) {
        if (k.startsWith('TEST_')) {
          env[k] = nodeEnv[k]
        }
      }
      window.testground = { env }
    }, process.env)

    console.log('opening up testplan webpage on localhost')
    await page.goto('http://127.0.0.1:8080')

    // TODO: wait for the test to finish somehow :|

    console.log('start browser exit process...')

    if (process.env.HALT_BROWSER_ON_FINISH === 'true') {
      console.log('halting on browser exit process (dev tools breakpoint)...')
      await page.evaluate(() => {
        debugger // eslint-disable-line no-debugger
        window.open() // triggers popup window
      })
      await page.waitForEvent('popup', { timeout: 0 })
    }
  } catch (error) {
    console.error(`browser process resulted in exception: ${error}`)
    throw error
  } finally {
    if (browser) {
      try {
        await browser.close()
      } catch (error) {
        console.error(`browser closure resulted in exception: ${error}`)
      }
    }
    console.log('exiting browser testplan...')
    exit(0)
  }
})()
