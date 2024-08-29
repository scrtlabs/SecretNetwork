//go:build linux && !muslc && arm64 && !sys_sgx_wrapper && !nosgx

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lsgx_wrapper_v1.0.3.aarch64
import "C"
