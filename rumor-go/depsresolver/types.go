package depsresolver

import "reflect"

type DepsResolver interface {
	SetPredefinedState(interface{})
	Emit(interface{}) error

	GetState() map[string]interface{}
	Resolve(reflect.Type) interface{}
	ResolveLatest(reflect.Type) interface{}
	Dispose()
}
