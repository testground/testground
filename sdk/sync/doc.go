// Package sync contains the test instance synchronisation service.
//
// This service allows instances belonging to a test case to coordinate with one
// another, by:
//
//  (1) discovering each other and learning their multiaddrs,
//  (2) communicating dynamically computed values needed for the test scenario,
//      e.g. CIDs of random files generated in the test,
//  (3) signalling/beaconing state, e.g. "I've reached state A in the FSM; await
//      until N other nodes have too".
//
package sync
