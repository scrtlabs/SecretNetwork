//go:build !secretcli && linux && !muslc && !darwin && !sgx
// +build !secretcli,linux,!muslc,!darwin,!sgx

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm -lsgx_dcap_ql -lsgx_dcap_quoteverify
import "C"
