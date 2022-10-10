const express = require('express')
const path = require('path')

module.exports = (runenv, _client, port) => {
  const app = express()

  app.use(express.static(path.join(__dirname, 'static')))

  return new Promise((resolve) => {
    app.listen(port, () => {
      runenv.recordMessage(`local web server running on port ${port}`)
      resolve()
    })
  })
}
