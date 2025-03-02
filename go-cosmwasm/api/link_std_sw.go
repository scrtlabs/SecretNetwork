//go:build !secretcli && linux && !muslc && !darwin && sw
// +build !secretcli,linux,!muslc,!darwin,sw

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm -lsgx_dcap_ql -lsgx_dcap_quoteverify
import "C"
