package keeper

import (
	"bytes"
	"compress/gzip"
	"io"

	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

// magic bytes to identify gzip.
// See https://www.ietf.org/rfc/rfc1952.txt
// and https://github.com/golang/go/blob/master/src/net/http/sniff.go#L186
var gzipIdent = []byte("\x1F\x8B\x08")

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

	return io.ReadAll(io.LimitReader(zr, types.MaxWasmSize))
}
