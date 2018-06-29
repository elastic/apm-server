package tests

import (
	"fmt"
	"regexp"
)

type Set struct {
	entries map[interface{}]interface{}
}

func NewSet(entries ...interface{}) *Set {
	s := Set{entries: map[interface{}]interface{}{}}
	for _, v := range entries {
		s.Add(v)
	}
	return &s
}

func (s *Set) Add(input interface{}) {
	s.entries[input] = nil
}

func (s *Set) Remove(input interface{}) {
	delete(s.entries, input)
}

func (s *Set) Contains(input interface{}) bool {
	if _, ok := s.entries[input]; ok {
		return true
	}
	return false
}

func (s *Set) ContainsStrPattern(str string) bool {
	if s.Contains(str) {
		return true
	}
	for _, entry := range s.Array() {
		if entryStr, ok := entry.(string); ok {
			re, err := regexp.Compile(fmt.Sprintf("^%s$", entryStr))
			if err == nil && re.MatchString(str) {
				return true
			}
		}
	}
	return false
}

func (s *Set) Copy() *Set {
	copy := NewSet()
	for k, _ := range s.entries {
		copy.Add(k)
	}
	return copy
}

func (s *Set) Len() int {
	return len(s.entries)
}

func Union(s1, s2 *Set) *Set {
	s := s1.Copy()
	for k, _ := range s2.entries {
		s.Add(k)
	}
	return s
}

func Difference(s1, s2 *Set) *Set {
	s := NewSet()
	for k, _ := range s1.entries {
		if !s2.Contains(k) {
			s.Add(k)
		}
	}
	return s
}

func SymmDifference(s1, s2 *Set) *Set {
	return Union(Difference(s1, s2), Difference(s2, s1))
}

func (s *Set) Array() []interface{} {
	if s == nil {
		return nil
	}
	a := make([]interface{}, 0, len(s.entries))
	for k, _ := range s.entries {
		a = append(a, k)
	}
	return a
}
