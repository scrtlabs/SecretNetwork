package keeper

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
)

// magic bytes to identify gzip.
// See https://www.ietf.org/rfc/rfc1952.txt
// and https://github.com/golang/go/blob/master/src/net/http/sniff.go#L186
var gzipIdent = []byte("\x1F\x8B\x08")

// limit max bytes read to prevent gzip bombs
const maxSize = 400 * 1024

// uncompress returns gzip uncompressed content or given src when not gzip.
func uncompress(src []byte) ([]byte, error) {
	if len(src) < 3 {
		return src, nil
	}
	if !bytes.Equal(gzipIdent, src[0:3]) {
		return src, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	zr.Multistream(false)

	return ioutil.ReadAll(io.LimitReader(zr, maxSize))
}

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
	} else {
		return false
	}
}
