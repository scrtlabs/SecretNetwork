package main

import (
	"os"
)

func main() {
	rootCmd, _ := NewRootCmd()
	if err := Execute(rootCmd); err != nil {
		os.Exit(1)
	}
}
