//go:build linux && muslc && !nosgx
// +build linux,muslc,!nosgx

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm_muslc
import "C"
