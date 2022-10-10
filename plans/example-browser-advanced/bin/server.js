const spawnServer = require('../server')

spawnServer({
  recordMessage: console.log
}, null, 8080)
