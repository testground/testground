// Package daemon provides a type-safe client for the Testground server. Testground uses a client-server architecture.
// The Testground client talks to the Testground daemon, which builds, and runs your test plans. The Testground client
// and daemon can run on the same system, or you can connect a Testground client to a remote Testground daemon.
// Currently all commands to Testground, but the `daemon` command, are client-side commands.
package daemon
