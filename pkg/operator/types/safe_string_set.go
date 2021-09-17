package types

import (
	"fmt"
	"sync"

	"github.com/jjeffery/stringset"
)

type SafeStringSet struct {
	set *stringset.Set
	mux sync.RWMutex
}

func NewSafeStringSet(v ...string) *SafeStringSet {
	set := &SafeStringSet{
		set: new(stringset.Set),
	}

	set.Add(v...)

	return set
}

func (s *SafeStringSet) Add(value ...string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.set.Add(value...)
}

func (s *SafeStringSet) Remove(value ...string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	s.set.Remove(value...)
}

func (s *SafeStringSet) Len() int {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.Len()
}

func (s *SafeStringSet) Contains(v string) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.Contains(v)
}

func (s *SafeStringSet) Equal(o *SafeStringSet) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.Equal(*o.set)
}

func (s *SafeStringSet) Values() []string {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.Values()
}

func (s *SafeStringSet) Join(sep string) string {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.Join(sep)
}

func (s *SafeStringSet) String() string {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.String()
}

func (s *SafeStringSet) GoString() string {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.set.GoString()
}

func (s *SafeStringSet) Format(f fmt.State, c rune) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	s.set.Format(f, c)
}
