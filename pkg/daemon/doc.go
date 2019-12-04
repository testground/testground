// Package daemon provides a type-safe client for the TestGround server. TestGround uses a client-server architecture.
// The TestGround client talks to the TestGround daemon, which builds, and runs your test plans. The TestGround client
// and daemon can run on the same system, or you can connect a TestGround client to a remote TestGround daemon.
// Currently all commands to TestGround, but the `daemon` command, are client-side commands.
package daemon
