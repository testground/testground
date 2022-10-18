//go:build foo && bar
// +build foo,bar

package main

/*
this is garbage.
when the build tags are activated, this file will detonate the build.
*/
