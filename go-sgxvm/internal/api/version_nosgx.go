//go:build nosgx
// +build nosgx

package api

/*
#include "bindings.h"
*/
import "C"

func Libsgx_wrapperVersion() (string, error) {
	return "", nil
}
