package depsresolver

import (
	"reflect"
	"sync"
	"testing"
)

type TestEntity struct {
	Foo string
}

func TestEventBus(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	bus := NewDepsResolver()

	// define event
	event := &TestEntity{
		Foo: "Bar",
	}

	// subscriber 1
	go func() {
		target := bus.Resolve(reflect.TypeOf((*TestEntity)(nil))).(*TestEntity)

		if target.Foo != event.Foo {
			panic("subscriber 1 result assertion failed")
		}

		wg.Done()
	}()

	// subscriber 2
	go func() {
		target := bus.Resolve(reflect.TypeOf((*TestEntity)(nil))).(*TestEntity)

		if target.Foo != event.Foo {
			panic("subscriber 2 result assertion failed")
		}

		wg.Done()
	}()

	// emit
	bus.Emit(event)

	wg.Wait()

	// subscriber 3, which should grab data directly from published cache
	directTarget := bus.Resolve(reflect.TypeOf((*TestEntity)(nil))).(*TestEntity)
	if directTarget.Foo != event.Foo {
		panic("subscriber 3 result assertion failed")
	}
}
