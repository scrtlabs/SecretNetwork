//go:build !secretcli && linux && !muslc && !darwin && !test
// +build !secretcli,linux,!muslc,!darwin,!test

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm -lsgx_uae_service -lsgx_dcap_ql -lsgx_dcap_quoteverify
import "C"
