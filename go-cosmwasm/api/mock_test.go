package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// 	"github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/types"
)

/*** Mock KVStore ****/

type Lookup struct {
	data map[string]string
}

func NewLookup() *Lookup {
	return &Lookup{data: make(map[string]string)}
}

func (l *Lookup) Get(key []byte) []byte {
	val := l.data[string(key)]
	return []byte(val)
}

func (l *Lookup) Set(key, value []byte) {
	l.data[string(key)] = string(value)
}

var _ KVStore = (*Lookup)(nil)

/***** Mock GoAPI ****/

const CanonicalLength = 42

func MockCanonicalAddress(human string) ([]byte, error) {
	if len(human) > CanonicalLength {
		return nil, fmt.Errorf("human encoding too long")
	}
	res := make([]byte, CanonicalLength)
	copy(res, []byte(human))
	return res, nil
}

func MockHumanAddress(canon []byte) (string, error) {
	if len(canon) != CanonicalLength {
		return "", fmt.Errorf("wrong canonical length")
	}
	cut := CanonicalLength
	for i, v := range canon {
		if v == 0 {
			cut = i
			break
		}
	}
	human := string(canon[:cut])
	return human, nil
}

func NewMockAPI() *GoAPI {
	return &GoAPI{
		HumanAddress:     MockHumanAddress,
		CanonicalAddress: MockCanonicalAddress,
	}
}

func TestMockApi(t *testing.T) {
	human := "foobar"
	canon, err := MockCanonicalAddress(human)
	require.NoError(t, err)
	assert.Equal(t, CanonicalLength, len(canon))

	recover, err := MockHumanAddress(canon)
	require.NoError(t, err)
	assert.Equal(t, recover, human)
}
