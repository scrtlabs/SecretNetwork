package main

import (
	"fmt"
	"io/ioutil"
	"os"

	wasm "github.com/enigmampc/SecretNetwork/go-cosmwasm"
)

// This is just a demo to ensure we can compile a static go binary
func main() {
	file := os.Args[1]
	fmt.Printf("Running %s...\n", file)
	bz, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	fmt.Println("Loaded!")

	os.MkdirAll("tmp", 0755)
	wasmer, err := wasm.NewWasmer("tmp", "staking", 0, 15)
  
	if err != nil {
		panic(err)
	}

	id, err := wasmer.Create(bz)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Got code id: %X\n", id)

	wasmer.Cleanup()
	fmt.Println("finished")
}
