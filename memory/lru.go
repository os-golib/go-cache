package memory

import "container/list"

// LRU implements a thread-safe LRU cache
type LRU[K comparable, V any] struct {
	capacity int
	list     *list.List
	items    map[K]*list.Element
}

// entry holds the key-value pair in the LRU
type entry[K comparable, V any] struct {
	key   K
	value V
}

// NewLRU creates a new LRU cache
func NewLRU[K comparable, V any](capacity int) *LRU[K, V] {
	return &LRU[K, V]{
		capacity: capacity,
		list:     list.New(),
		items:    make(map[K]*list.Element),
	}
}

// Get retrieves a value from the cache
func (l *LRU[K, V]) Get(key K) (V, bool) {
	if elem, ok := l.items[key]; ok {
		l.list.MoveToFront(elem)
		return elem.Value.(*entry[K, V]).value, true
	}
	var zero V
	return zero, false
}

// Set adds a value to the cache
func (l *LRU[K, V]) Set(key K, value V) {
	if elem, ok := l.items[key]; ok {
		l.list.MoveToFront(elem)
		elem.Value.(*entry[K, V]).value = value
		return
	}

	if l.list.Len() == l.capacity {
		l.evict()
	}

	elem := l.list.PushFront(&entry[K, V]{key, value})
	l.items[key] = elem
}

// evict removes the least recently used item
func (l *LRU[K, V]) evict() {
	elem := l.list.Back()
	if elem != nil {
		l.list.Remove(elem)
		delete(l.items, elem.Value.(*entry[K, V]).key)
	}
}

// Len returns the number of items in the cache
func (l *LRU[K, V]) Len() int {
	return l.list.Len()
}

// Clear removes all items from the cache
func (l *LRU[K, V]) Clear() {
	l.list.Init()
	l.items = make(map[K]*list.Element)
}
