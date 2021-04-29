package depsresolver

import (
	"fmt"
	"reflect"
	"sync"
)

type DepsResolverInstance struct {
	mux       sync.RWMutex
	rmux      sync.RWMutex
	channels  map[reflect.Type][]chan interface{}
	published map[reflect.Type]interface{}
	latest    map[reflect.Type]interface{}
}

func NewDepsResolver() DepsResolver {
	return &DepsResolverInstance{
		mux:       sync.RWMutex{},
		rmux:      sync.RWMutex{},
		channels:  make(map[reflect.Type][]chan interface{}),
		published: make(map[reflect.Type]interface{}),
		latest:    make(map[reflect.Type]interface{}),
	}
}

func (resolver *DepsResolverInstance) SetPredefinedState(entity interface{}) {
	event := getEvent(entity)

	resolver.rmux.Lock()
	resolver.published[event] = entity
	resolver.latest[event] = entity
	resolver.rmux.Unlock()
}

func (resolver *DepsResolverInstance) Emit(entity interface{}) error {
	resolver.rmux.Lock()
	event := getEvent(entity)

	if _, alreadyEmitted := resolver.published[event]; alreadyEmitted {
		return fmt.Errorf("cannot commit same entity more than once")
	}

	var emitSource interface{}

	resolver.latest[event] = entity
	resolver.published[event] = entity
	emitSource = resolver.published[event]

	resolver.rmux.Unlock()

	resolver.mux.Lock()
	if len(resolver.channels[event]) != 0 {
		for _, subscription := range resolver.channels[event] {
			subscription <- emitSource
		}
	}
	resolver.mux.Unlock()

	return nil
}

func (resolver *DepsResolverInstance) GetState() map[string]interface{} {
	state := map[string]interface{}{}
	resolver.rmux.RLock()
	for key, entity := range resolver.published {
		if isEntityZero(entity) {
			continue
		}
		state[key.Name()] = entity
	}

	resolver.rmux.RUnlock()

	return state
}

// Resolver is really just a subscriber
func (resolver *DepsResolverInstance) Resolve(event reflect.Type) interface{} {
	// check if this event has been delivered already
	// in such case, get data directly from resolver.published
	resolver.rmux.RLock()

	if entity, ok := resolver.published[event]; ok {
		resolver.rmux.RUnlock()
		return entity
	}

	// otherwise start polling on the event channel
	subchannel := make(chan interface{})
	resolver.mux.Lock()
	resolver.channels[event] = append(resolver.channels[event], subchannel)
	resolver.mux.Unlock()
	resolver.rmux.RUnlock()

	select {
	case e := <-subchannel:
		return e
	}
}

func (resolver *DepsResolverInstance) ResolveLatest(event reflect.Type) interface{} {
	return resolver.latest[event]
}

func (resolver *DepsResolverInstance) Dispose() {
	resolver.rmux.Lock()
	for _, entity := range resolver.channels {
		for _, channel := range entity {
			close(channel)
		}
	}

	resolver.channels = make(map[reflect.Type][]chan interface{})

	// and dispose the previous published data
	resolver.published = make(map[reflect.Type]interface{})
	resolver.rmux.Unlock()
}

func getEvent(entity interface{}) reflect.Type {
	t := reflect.TypeOf(entity)

	switch t.Kind() {
	case reflect.Ptr:
		return t.Elem()
	case reflect.Struct, reflect.Slice, reflect.Map:
		return t
	default:
		panic("Invalid type entity provided")
	}
}

func isEntityZero(entity interface{}) bool {
	if entity == nil {
		return true
	}

	ref := reflect.Indirect(reflect.ValueOf(entity))

	switch ref.Kind() {
	case reflect.Ptr:
		return ref.IsNil()
	case reflect.Struct:
		return ref.IsZero()
	case reflect.Slice, reflect.Array, reflect.Map:
		return ref.IsNil() || (ref.Len() == 0)
	default:
		return ref.IsZero()
	}
}
