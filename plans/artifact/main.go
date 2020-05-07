package main

import (
	"github.com/ipfs/testground/sdk/runtime"
	"io/ioutil"
)

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	buf, err := ioutil.ReadFile("/artifacts/test.txt")
	if err != nil {
		return err
	}
	runenv.Message(string(buf))
	return nil
}
