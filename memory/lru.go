package memory

import (
	"container/list"
	"sync"
)

// entry holds the key-value pair in the LRU
type entry[K comparable, V any] struct {
	key   K
	value V
}

// LRU implements a thread-safe LRU cache without expiration
type LRU[K comparable, V any] struct {
	capacity int
	list     *list.List
	items    map[K]*list.Element
	mu       sync.RWMutex
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
	l.mu.Lock()
	defer l.mu.Unlock()

	elem, ok := l.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	l.list.MoveToFront(elem)
	return elem.Value.(*entry[K, V]).value, true
}

// Set adds or updates a value in the cache
func (l *LRU[K, V]) Set(key K, value V) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Update existing
	if elem, ok := l.items[key]; ok {
		l.list.MoveToFront(elem)
		elem.Value.(*entry[K, V]).value = value
		return
	}

	// Evict if at capacity
	if l.list.Len() == l.capacity {
		l.evict()
	}

	// Add new
	elem := l.list.PushFront(&entry[K, V]{key, value})
	l.items[key] = elem
}

// Delete removes a key from the cache
func (l *LRU[K, V]) Delete(key K) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	elem, ok := l.items[key]
	if !ok {
		return false
	}

	l.list.Remove(elem)
	delete(l.items, key)
	return true
}

// Len returns the number of items in the cache
func (l *LRU[K, V]) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.list.Len()
}

// Clear removes all items from the cache
func (l *LRU[K, V]) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.list.Init()
	l.items = make(map[K]*list.Element)
}

// evict removes the least recently used item (must hold write lock)
func (l *LRU[K, V]) evict() {
	elem := l.list.Back()
	if elem != nil {
		l.list.Remove(elem)
		delete(l.items, elem.Value.(*entry[K, V]).key)
	}
}

// Peek retrieves a value without updating access time
func (l *LRU[K, V]) Peek(key K) (V, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	elem, ok := l.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	return elem.Value.(*entry[K, V]).value, true
}

// Keys returns all keys in the cache (from most to least recently used)
func (l *LRU[K, V]) Keys() []K {
	l.mu.RLock()
	defer l.mu.RUnlock()

	keys := make([]K, 0, l.list.Len())
	for elem := l.list.Front(); elem != nil; elem = elem.Next() {
		keys = append(keys, elem.Value.(*entry[K, V]).key)
	}
	return keys
}
