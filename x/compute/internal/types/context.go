package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const contextKeyCallDepth contextKey = iota

func WithCallDepth(ctx sdk.Context, counter uint32) sdk.Context {
	return ctx.WithValue(contextKeyCallDepth, counter)
}

func CallDepth(ctx sdk.Context) (uint32, bool) {
	val, ok := ctx.Value(contextKeyCallDepth).(uint32)
	return val, ok
}
