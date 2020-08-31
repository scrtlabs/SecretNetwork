package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageInfoHandlesMultipleCoins(t *testing.T) {
	info := MessageInfo{
		Sender: "foobar",
		SentFunds: []Coin{
			{Denom: "peth", Amount: "12345"},
			{Denom: "uatom", Amount: "789876"},
		},
	}
	bz, err := json.Marshal(info)
	require.NoError(t, err)

	// we can unmarshal it properly into struct
	var recover MessageInfo
	err = json.Unmarshal(bz, &recover)
	require.NoError(t, err)
	assert.Equal(t, info, recover)
}

func TestMessageInfoHandlesMissingCoins(t *testing.T) {
	info := MessageInfo{
		Sender: "baz",
	}
	bz, err := json.Marshal(info)
	require.NoError(t, err)

	// we can unmarshal it properly into struct
	var recover MessageInfo
	err = json.Unmarshal(bz, &recover)
	require.NoError(t, err)
	assert.Equal(t, info, recover)

	// make sure "sent_funds":[] is in JSON
	var raw map[string]json.RawMessage
	err = json.Unmarshal(bz, &raw)
	require.NoError(t, err)
	sent, ok := raw["sent_funds"]
	require.True(t, ok)
	assert.Equal(t, string(sent), "[]")
}
