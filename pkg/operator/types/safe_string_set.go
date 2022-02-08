package types

import (
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

type SafeStringSet struct {
	set sets.String
	mux sync.RWMutex
}

func NewSafeStringSet(v ...string) *SafeStringSet {
	set := &SafeStringSet{
		set: make(sets.String),
	}

	set.Insert(v...)

	return set
}

func (s *SafeStringSet) Insert(value ...string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.set.Insert(value...)
}

func (s *SafeStringSet) Delete(value ...string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	s.set.Delete(value...)
}

func (s *SafeStringSet) Len() int {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.Len()
}

func (s *SafeStringSet) Has(v string) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.Has(v)
}

func (s *SafeStringSet) Equal(o *SafeStringSet) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.Equal(o.set)
}

func (s *SafeStringSet) List() []string {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.List()
}
