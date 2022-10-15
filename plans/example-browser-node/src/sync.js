const { sync, network } = require('@testground/sdk')

function sleep (ms) {
  return new Promise((resolve) => {
    setTimeout(resolve, ms)
  })
}

function getRandom (min, max) {
  return Math.random() * (max - min) + min
}

// Demonstrates synchronization between instances in the test group.
//
// In this example, the first instance to signal enrollment becomes the leader
// of the test case.
//
// The leader waits until all the followers have reached the state "ready"
// then, the followers wait for a signal from the leader to be released.
module.exports = async (runenv, client) => {
  const enrolledState = 'enrolled'
  const readyState = 'ready'
  const releasedState = 'released'

  // instantiate a network client; see 'Traffic shaping' in the docs.
  const netclient = network.newClient(client, runenv)
  runenv.recordMessage('waiting for network initialization')

  // wait for the network to initialize; this should be pretty fast.
  netclient.waitNetworkInitialized()
  runenv.recordMessage('network initilization complete')

  // signal entry in the 'enrolled' state, and obtain a sequence number.
  const seq = await client.signalEntry(enrolledState)
  runenv.recordMessage(`my sequence ID: ${seq}`)

  // if we're the first instance to signal, we'll become the LEADER.
  if (seq === 1) {
    runenv.recordMessage('i am the leader.')
    const numFollowers = runenv.testInstanceCount - 1

    // let's wait for the followers to signal.
    runenv.recordMessage(`waiting for ${numFollowers} instances to become ready`)
    const b = await client.barrier(readyState, numFollowers)
    await b.wait

    runenv.recordMessage('the followers are all ready')
    runenv.recordMessage('ready...')
    await sleep(1000) // 1s
    runenv.recordMessage('set...')
    await sleep(5000) // 5s
    runenv.recordMessage('go, release followers!')

    // signal on the 'released' state.
    await client.signalEntry(releasedState)
    return
  }

  const ms = getRandom(1, 5) * 1000
  runenv.recordMessage(`i am a follower; signalling ready after ${ms} milliseconds`)

  await sleep(ms)

  runenv.recordMessage('follower signalling now')

  // signal entry in the 'ready' state.
  await client.signalEntry(readyState)

  // wait until the leader releases us.
  const b = await client.barrier(releasedState, 1)
  await b.wait

  runenv.recordMessage('i have been released')
}
