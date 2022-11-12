const { chromium, firefox, webkit } = require('playwright')
const { exit } = require('process')
const { spawn } = require('child_process')

const spawnServer = require('./server')

;(async () => {
  spawnServer(8080)

  let browser
  try {
    const browserDebugPort = process.env.TEST_BROWSER_DEBUG_PORT || 9222

    switch (process.env.TEST_BROWSER_KIND || 'chromium') {
      case 'chromium':
        console.log(`launching chromium browser with exposed debug port: ${browserDebugPort}`)
        browser = await chromium.launch({
          args: [
            '--remote-debugging-address=0.0.0.0',
            `--remote-debugging-port=${browserDebugPort}`
          ]
        })
        break

      case 'firefox':
        const localBrowserDebugPort = Number(browserDebugPort) + 1
        console.log(`launching firefox browser with exposed debug port: ${browserDebugPort} (local ${localBrowserDebugPort})`)
        browser = await firefox.launch({
          args: [
            `-start-debugger-server=${localBrowserDebugPort}`
          ]
        })

        console.log('launching tcp proxy to expose firefox debugger for remote access')
        const tcpProxy = spawn(
          'socat', [
            `tcp-listen:${browserDebugPort},bind=0.0.0.0,fork`,
            `tcp:localhost:${localBrowserDebugPort}`
          ]
        )
        tcpProxy.stdout.on('data', (data) => {
          console.log(`firefox debugger: tcpProxy: stdout: ${data}`)
        })

        tcpProxy.stderr.on('data', (data) => {
          console.error(`firefox debugger: tcpProxy: stderr: ${data}`)
        })

        break

      case 'webkit':
        console.log('launching webkit browser (remote debugging not yet supported)')
        browser = await webkit.launch()
        break
    }

    const page = await browser.newPage()

    page.on('console', (message) => {
      const loc = message.location()
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

    if (process.env.TEST_KEEP_OPENED_BROWSERS === 'true') {
      // TODO: create a better way, which actually does halt!
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
