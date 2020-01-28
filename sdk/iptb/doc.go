// Package iptb wraps IPTB in a nice API to create fixtures (ensembles)
// declaratively, and with inversion of control.
//
// This enables a nice DX, where the configuration for an IPTB ensemble is
// colocated with the test case logic itself, rather than scattered across
// source files.
//
// Test plans using this package instantiate a TestEnsembleSpec and they inject
// it into the test case. The test case then uses the spec's methods to declare
// the nodes it needs, associating each with a tag. In the future, spec'ing will
// be more versatile; test cases will be able to provide IPFS configurations for
// each node as well.
//
// Once the specification work is over, the ensemble can be initialized by
// calling the Healthcheck() method.
//
// This creates the desired IPTB testbed and encapsulates it within a
// TestEnsemble with friendly accessors to grab the instantiated nodes by tag,
// and acquire HTTP clients to operate on them.
package iptb
