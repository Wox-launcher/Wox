package util

import (
	"encoding/json"
	"fmt"
	"sync"
)

type HashMap[K comparable, V any] struct {
	inner map[K]V
	rw    sync.RWMutex
}

func NewHashMap[K comparable, V any]() *HashMap[K, V] {
	return &HashMap[K, V]{
		inner: make(map[K]V),
	}
}

func (h *HashMap[K, V]) UnmarshalJSON(b []byte) error {
	h.inner = make(map[K]V)
	json.Unmarshal(b, &h.inner)
	return nil
}

func (h *HashMap[K, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.inner)
}

func (h *HashMap[K, V]) Store(k K, v V) {
	h.rw.Lock()
	defer h.rw.Unlock()

	h.inner[k] = v
}

func (h *HashMap[K, V]) Exist(k K) bool {
	h.rw.RLock()
	defer h.rw.RUnlock()

	_, ok := h.inner[k]
	return ok
}

func (h *HashMap[K, V]) NotExist(k K) bool {
	return !h.Exist(k)
}

func (h *HashMap[K, V]) Load(k K) (V, bool) {
	h.rw.RLock()
	defer h.rw.RUnlock()

	v, ok := h.inner[k]
	return v, ok
}

func (h *HashMap[K, V]) Clear() {
	h.rw.Lock()
	defer h.rw.Unlock()

	h.inner = make(map[K]V)
}

func (h *HashMap[K, V]) Delete(k K) {
	h.rw.Lock()
	defer h.rw.Unlock()

	delete(h.inner, k)
}

func (h *HashMap[K, V]) Len() (length int) {
	h.rw.RLock()
	defer h.rw.RUnlock()

	return len(h.inner)
}

func (h *HashMap[K, V]) MustLoad(k K) V {
	v, ok := h.Load(k)
	if !ok {
		panic(fmt.Sprintf("key %v not exist", k))
	}
	return v
}

func (h *HashMap[K, V]) Range(f func(K, V) bool) {
	h.rw.RLock()
	defer h.rw.RUnlock()

	for k, v := range h.inner {
		if !f(k, v) {
			break
		}
	}
}

func (h *HashMap[K, V]) String() (s string) {
	h.Range(func(k K, v V) bool {
		s += fmt.Sprintf("{%+v:%+v}, ", k, v)
		return true
	})

	return s
}

func (h *HashMap[K, V]) ToMap() (m map[K]V) {
	h.Range(func(k K, v V) bool {
		m[k] = v
		return true
	})

	return m
}
