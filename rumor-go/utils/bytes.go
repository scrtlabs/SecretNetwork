package utils

import (
	"bytes"
)

func ConcatBytes(items ...[]byte) []byte {
	buf := bytes.NewBuffer(nil)
	for _, item := range items {
		buf.Write(item)
	}
	return buf.Bytes()
}
