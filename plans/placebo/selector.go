//go:build foo && bar
// +build foo,bar

package main

func main() {
	garbage()
}
