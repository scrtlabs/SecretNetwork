package keeper

import (
	"io/ioutil"
	"os"
)

func getFile(src string) ([]byte, error) {
	file, err := os.Open(src)
	if err != nil {
		// log.Fatal(err)
		return nil, err
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	return b, err
}

func fileExists(src string) bool {
	if _, err := os.Stat(src); err == nil {
		return true
	}
	return false
}
