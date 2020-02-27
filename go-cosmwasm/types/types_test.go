package types

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCanoncialAddress(t *testing.T) {
	addr := CanonicalAddress([]byte{17, 121, 0, 93})
	bz, err := json.Marshal(addr)
	if err != nil {
		t.Error(err)
	}
	if string(bz) != "[17,121,0,93]" {
		t.Errorf("Unexpected encoding %s", string(bz))
	}

	var load CanonicalAddress
	err = json.Unmarshal(bz, &load)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(addr, load) {
		t.Errorf("Unexpected encoding %X", load)
	}
}
