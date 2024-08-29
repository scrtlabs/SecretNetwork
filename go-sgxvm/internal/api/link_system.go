//go:build sys_sgx_wrapper && !nosgx

package api

// #cgo LDFLAGS: -lsgx_wrapper_v1.0.3
import "C"
