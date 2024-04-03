//go:build !secretcli && linux && !muslc && !darwin && test
// +build !secretcli,linux,!muslc,!darwin,test

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm
import "C"
