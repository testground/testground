const os = require('os')
const net = require('net')
const ipaddr = require('ipaddr.js')
const { performance } = require('perf_hooks')
const { sync, network } = require('@testground/sdk')


async function pingpong (runenv, client) {
  runenv.recordMessage('before sync.newBoundClient')

  if (!runenv.testSidecar) {
    throw new Error('this test requires a sidecar.')
  }

  let server, socket

  try {
    const netclient = network.newClient(client, runenv)

    runenv.recordMessage('before netclient.waitNetworkInitialized')
    await netclient.waitNetworkInitialized()

    const oldAddrs = os.networkInterfaces()

    const config = {
      network: 'default',
      enable: true,
      default: {
        latency: 100 * 1000 * 1000, // 100ms in nanoseconds
        bandwidth: 1 << 20 // 1 Mib
      },
      callbackState: 'network-configured',
      routingPolicy: network.DENY_ALL
    }

    runenv.recordMessage('before netclient.configureNetwork')
    await netclient.configureNetwork(config)

    const seq = await client.signalAndWait('ip-allocation', runenv.testInstanceCount)

    // Make sure that the IP addresses don't change unless we request it.
    const newAddrs = os.networkInterfaces()
    if (!sameAddrs(oldAddrs, newAddrs)) {
      throw new Error('interfaces changed')
    }

    runenv.recordMessage(`I am ${seq}`)

    const ip = [...runenv.testSubnet[0].octets.slice(0, 2), 1, seq]

    config.IPv4 = `${ip.join('.')}/${runenv.testSubnet[1]}`
    config.callbackState = 'ip-changed'

    if (seq === 1) {
      server = net.createServer()
      server.listen(1234, '0.0.0.0')
    }

    runenv.recordMessage('before reconfiguring network')
    await netclient.configureNetwork(config)

    switch (seq) {
      case 1:
        socket = await new Promise(resolve => {
          server.once('connection', resolve)
        })

        runenv.recordMessage('server is connected!')
        break
      case 2:
        socket = new net.Socket()

        await new Promise(resolve => {
          const raw = ip.slice(0, 3)
          raw.push(1)
          const addr = ipaddr.fromByteArray(raw)
          socket.connect(1234, addr.toString(), resolve)
        })

        runenv.recordMessage('client is connected!')
        break
      default:
        throw new Error('expected at most two test instances')
    }

    socket.setNoDelay(true)

    const write = (data) => new Promise(resolve => {
      socket.write(data, resolve)
    })

    const read = () => new Promise(resolve => {
      socket.once('data', (data) => {
        socket.pause()
        resolve(data)
      })
      socket.resume()
    })

    const pingPong = async (test, rttMin, rttMax) => {
      runenv.recordMessage('waiting until ready')

      // wait until both sides are ready
      await write('0')
      await read()

      const start = performance.now()

      // write sequence number
      runenv.recordMessage('writing my id')
      await write(seq.toString())

      // pong other sequence number
      runenv.recordMessage('reading their id')
      const theirs = await read()

      runenv.recordMessage('returning their id')
      await write(theirs)

      runenv.recordMessage('reading my id')
      const mine = await read()

      runenv.recordMessage('done')

      const end = performance.now()

      // check the sequence number.
      if (seq.toString() !== mine.toString()) {
        throw Error(`read unexpected value, expected ${seq}, received ${mine}`)
      }

      // check the RTT
      const rtt = end - start
      if (rtt < rttMin || rtt > rttMax) {
        throw new Error(`expected an RTT between ${rttMin} and ${rttMax}, got ${rtt}`)
      }

      runenv.recordMessage(`ping RTT was ${rtt} [${rttMin}, ${rttMax}]`)

      // Don't reconfigure the network until we're done with the first test.
      await client.signalAndWait(`ping-pong-${test}`, runenv.testInstanceCount)
    }

    await pingPong('200', 200, 215)

    config.default.latency = 10 * 1000 * 1000 // 10ms in nanoseconds
    config.callbackState = 'latency-reduced'
    await netclient.configureNetwork(config)

    runenv.recordMessage('ping pong')
    await pingPong('10', 20, 35)
  } finally {
    if (server) server.close()
    if (socket) socket.destroy()
  }
}

function sameAddrs (a, b) {
  if (a.length !== b.length) {
    return false
  }

  a = Object.values(a)
    .flat()
    .reduce((acc, curr) => {
      if (!acc.includes(curr.cidr)) {
        acc.push(curr.cidr)
      }
      return acc
    }, [])

  b = Object.values(b).flat()

  for (const { cidr } of b) {
    if (!a.includes(cidr)) {
      return false
    }
  }

  return true
}

module.exports = pingpong
