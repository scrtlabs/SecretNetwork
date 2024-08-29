//go:build linux && !muslc && amd64 && !sys_sgx_wrapper && !nosgx && attestationServer

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lsgx_attestation_wrapper_v1.0.3.x86_64
import "C"
