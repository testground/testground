// Package auto contains functions and types related to automation in response
// to repository events. In practice, this is the GitHub integration.
//
// Testground scheduling should be configured by a rulebook that defines what
// test plans or test cases are launched when, e.g. on new commits to `master`
// of repo go-ipfs, launch X test plans; on commits on branches of any repo in
// the libp2p org, launch X test plan; etc.
//
// This rulebook should be specified in a TOML configuration file, passed to the
// testground daemon, e.g.:
//
//   testground daemon --config testground.toml
//
// This package is also responsible for reacting to GitHub comments from
// befriended developers, e.g.
//
//   @testbot run <testplan> with <dependency=gitref> <dependency=gitref> <dependency=gitref>
//
// As well as maintaining that developer whitelist.
package auto
