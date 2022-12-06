const path = require('path')
const express = require('express')

/**
 * Exposes a minimal express server
 * on the desired port, serving the static folder as is.
 *
 * Only goal of this server is to serve the testplan together
 * with the `@testground/sdk` in the selected browser environment.
 */
module.exports = (port) => {
  const app = express()

  // Besides the minimal `index.html` page,
  // this folder also contains the `plan.bundle.js` file (once bundled with WebPack),
  // containing your test plan, together with the browser-enabled `@testground/sdk`
  // and any other dependencies this SDK or your test plan uses.
  app.use(express.static(path.join(__dirname, 'static')))

  return new Promise((resolve) => {
    app.listen(port, () => {
      console.log(`local web server running on port ${port}`)
      resolve()
    })
  })
}
