package types

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDelegationWithEmptyArray(t *testing.T) {
	var del Delegations
	bz, err := json.Marshal(&del)
	require.NoError(t, err)
	assert.Equal(t, string(bz), `[]`)

	var redel Delegations
	err = json.Unmarshal(bz, &redel)
	require.NoError(t, err)
	assert.Nil(t, redel)
}

func TestDelegationWithData(t *testing.T) {
	del := Delegations{{
		Validator: "foo",
		Delegator: "bar",
		Amount:    NewCoin(123, "stake"),
	}}
	bz, err := json.Marshal(&del)
	require.NoError(t, err)

	var redel Delegations
	err = json.Unmarshal(bz, &redel)
	require.NoError(t, err)
	assert.Equal(t, redel, del)
}

func TestValidatorWithEmptyArray(t *testing.T) {
	var val Validators
	bz, err := json.Marshal(&val)
	require.NoError(t, err)
	assert.Equal(t, string(bz), `[]`)

	var reval Validators
	err = json.Unmarshal(bz, &reval)
	require.NoError(t, err)
	assert.Nil(t, reval)
}

func TestValidatorWithData(t *testing.T) {
	val := Validators{{
		Address:       "1234567890",
		Commission:    "0.05",
		MaxCommission: "0.1",
		MaxChangeRate: "0.02",
	}}
	bz, err := json.Marshal(&val)
	require.NoError(t, err)

	var reval Validators
	err = json.Unmarshal(bz, &reval)
	require.NoError(t, err)
	assert.Equal(t, reval, val)
}
