// Package daemon provides a type-safe client for the Testground server. TestGround uses a client-server architecture.
// The Testground client talks to the TestGround daemon, which builds, and runs your test plans. The TestGround client
// and daemon can run on the same system, or you can connect a Testground client to a remote TestGround daemon.
// Currently all commands to Testground, but the `daemon` command, are client-side commands.
package daemon
