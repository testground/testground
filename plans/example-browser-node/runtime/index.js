const { chromium, firefox, webkit } = require('playwright')
const { exit } = require('process')
const { spawn } = require('child_process')

const { runtime } = require('@testground/sdk')

const spawnServer = require('./server')

/**
 * Runs your testplan — found in the src/ folder of this testplan —
 * as follows:
 *
 * 1. spawn the Express server containing the root index.html page
 *    and the plan.bundle.js file, containing your src/ testplan,
 *    @testground/sdk and any indirect or direct dependencies required;
 * 2. start playwright with the desired browser engine (chromium by default)
 * 3. inject the testground env parameters into the browser
 * 4. go to the start page to execute the `plan.bundle.js` file,
 *    and as a consequence start the selected testplan
 *    (selected via the exposed env variables)
 * 5. wait for the testplan to finish
 * 6. exit immediately, or hang the browser until a SIGINT signal
 *    in-case this opt-in feature was enabled.
 * 7. exit, fun and profit
 */
;(async () => {
  const envParameters = runtime.getEnvParameters()
  const runner = runtime.parseRunEnv(envParameters)

  const runtimeKind = runner.runParams.testInstanceParams['Runtime'] || 'node';
  if (runtimeKind === 'node') {
    require('../src/index')
    return
  }

  await spawnServer(8080)

  let browser
  try {
    const browserDebugPort =
      runner.runParams.testInstanceParams['BrowserDebugPort'] || process.env.TEST_BROWSER_DEBUG_PORT ||  9222

    switch (runtimeKind) {
      // chromium is the default browser engine,
      // and the only browser for which we currently support
      // remote debugging, meaning attaching a local chrome (on your host)
      // to the chromium version running in docker under a testground plan.
      case 'chromium':
        console.log(`launching chromium browser with exposed debug port: ${browserDebugPort}`)
        browser = await chromium.launch({
          headless: true,
          devtools: true,
          args: [
            '--remote-debugging-address=0.0.0.0',
            `--remote-debugging-port=${browserDebugPort}`
          ]
        })
        break

      // NOTE: remote debugging is not supported on webkit,
      // it should in theory be possible, but no working solution
      // has been demonstrated so far. Consider the remote debugging for this
      // browser engine (firefox) as a starting point should you want to contribute
      // such support yourself.
      case 'firefox':
        const localBrowserDebugPort = Number(browserDebugPort) + 1
        console.log(`launching firefox browser with exposed debug port: ${browserDebugPort} (local ${localBrowserDebugPort})`)
        browser = await firefox.launch({
          headless: true,
          devtools: true,
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

      // NOTE: remote debugging is not supported on webkit,
      // nor do we know of an approach on how we would allow such a thing
      case 'webkit':
        console.log('launching webkit browser (remote debugging not yet supported)')
        browser = await webkit.launch({
          headless: true,
          devtools: true
        })
        break

      default:
        console.error(`unknown runtime kind: ${runtimeKind}`)
        exit(1)
    }

    const page = await browser.newPage()

    page.on('console', (message) => {
      const loc = message.location()
      console.log(`[${message.type()}] ${loc.url}@L${loc.lineNumber}:C${loc.columnNumber}: ${message.text()} — ${message.args()}`)
    })

    console.log('prepare page window (global) environment')
    await page.addInitScript((env) => {
      window.testground = { env }
    }, envParameters)

    console.log('opening up testplan webpage on localhost')
    await page.goto('http://127.0.0.1:8080')

    console.log('waiting until testCase is finished...')
    // `window.testground.result` is set by @testground/sdk (js),
    // at the end of invokeMap
    const testgroundResult = await page.waitForFunction(() => {
      return window.testground && window.testground.result;
    });
    console.log(`testground in browser finished with result: ${testgroundResult}`)

    console.log('start browser exit process...')

    if (runner.runParams.testInstanceParams['KeepOpenedBrowsers'] === 'true' || process.env.TEST_KEEP_OPENED_BROWSERS === 'true') {
      console.log('halting browser until SIGINT is received...')
      await new Promise((resolve) => {
        process.on('SIGINT', resolve)
      })
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
