package util

import (
	"fmt"
	"sync"
)

type HashMap[K comparable, V any] struct {
	inner sync.Map
}

func (h *HashMap[K, V]) Store(k K, v V) {
	h.inner.Store(k, v)
}

func (h *HashMap[K, V]) Exist(k K) bool {
	if _, ok := h.inner.Load(k); ok {
		return true
	}

	return false
}

func (h *HashMap[K, V]) NotExist(k K) bool {
	return !h.Exist(k)
}

func (h *HashMap[K, V]) Load(k K) (V, bool) {
	if load, ok := h.inner.Load(k); ok {
		return load.(V), true
	}

	//nolint
	return *new(V), false
}

func (h *HashMap[K, V]) Clear() {
	h.inner.Range(func(key, _ any) bool {
		h.inner.Delete(key)
		return true
	})
}

func (h *HashMap[K, V]) Delete(k K) {
	h.inner.Delete(k)
}

func (h *HashMap[K, V]) Len() (length int64) {
	h.inner.Range(func(_, _ any) bool {
		length++
		return true
	})
	return
}

func (h *HashMap[K, V]) MustLoad(k K) V {
	if load, ok := h.inner.Load(k); ok {
		return load.(V)
	}

	panic(fmt.Sprintf("找不到%v的值", k))
}

func (h *HashMap[K, V]) Range(f func(K, V) bool) {
	h.inner.Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}

func (h *HashMap[K, V]) String() (s string) {
	h.Range(func(k K, v V) bool {
		s += fmt.Sprintf("{%+v:%+v}, ", k, v)
		return true
	})

	return s
}

func (h *HashMap[K, V]) ToMap() (m map[K]V) {
	m = make(map[K]V)
	h.Range(func(k K, v V) bool {
		m[k] = v
		return true
	})
	return
}
