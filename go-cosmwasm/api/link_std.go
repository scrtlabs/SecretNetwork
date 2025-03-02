//go:build !secretcli && linux && !muslc && !darwin && hw
// +build !secretcli,linux,!muslc,!darwin,hw

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm -lsgx_dcap_ql -lsgx_dcap_quoteverify -lsgx_epid
import "C"
