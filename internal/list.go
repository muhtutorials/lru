package internal

import "time"

// Entry is an LRU Entry
type Entry[K comparable, V any] struct {
	// Next and previous pointers in the doubly-linked list of elements.
	// To simplify the implementation, internally a list l is implemented
	// as a ring, such that &l.root is both the next element of the last
	// list element (l.Back()) and the previous element of the first list
	// element (l.Front()).
	next, prev *Entry[K, V]

	// The list to which this element belongs.
	list *LRUList[K, V]

	// The LRU Key of this element.
	Key K

	// The Value stored with this element.
	Value V

	// The time this element would be cleaned up (optional)
	ExpiresAt time.Time

	// The expiry bucket index this entry was put in (optional)
	Bucket uint8
}

// PrevEntry returns the previous list element or nil.
func (e *Entry[K, V]) PrevEntry() *Entry[K, V] {
	if p := e.prev; e.list != nil && p != &e.list.root {
		return p
	}
	return nil
}

// LRUList represents a doubly linked list.
// The zero value for LRUList is an empty list ready to use.
type LRUList[K comparable, V any] struct {
	root Entry[K, V] // sentinel list element, only &root, root.prev, and root.next are used
	len  int         // current list length excluding (this) sentinel element
}

// Init initializes or clears list l.
func (l *LRUList[K, V]) Init() *LRUList[K, V] {
	l.root.next = &l.root
	l.root.prev = &l.root
	l.len = 0
	return l
}

// lazyInit lazily initializes a zero List Value.
func (l *LRUList[K, V]) lazyInit() {
	if l.root.next == nil {
		l.Init()
	}
}

// NewList returns an initialized list.
func NewList[K comparable, V any]() *LRUList[K, V] {
	return new(LRUList[K, V]).Init()
}

// Len returns the number of elements of list l.
// The complexity is O(1).
func (l *LRUList[K, V]) Len() int {
	return l.len
}

// Back returns the last element of list l or nil if the list is empty.
func (l *LRUList[K, V]) Back() *Entry[K, V] {
	if l.len == 0 {
		return nil
	}
	return l.root.prev
}

// insert inserts e after at, increments l.len, and returns e.
func (l *LRUList[K, V]) insert(e, at *Entry[K, V]) *Entry[K, V] {
	//      <- elem ->
	// root <- root -> root
	e.prev = at
	// root <- elem ->
	// root <- root -> root
	e.next = at.next
	// root <- elem -> root
	// root <- root -> root
	e.prev.next = e
	// root <- elem -> root
	// root <- root -> elem
	e.next.prev = e
	// root <- elem -> root
	// elem <- root -> elem
	e.list = l
	l.len++
	return e
}

// insertValue is a convenience wrapper for insert(&Entry{Key: k, Value: v, ExpiresAt: ExpiresAt}, at).
func (l *LRUList[K, V]) insertValue(k K, v V, expiresAt time.Time, at *Entry[K, V]) *Entry[K, V] {
	return l.insert(&Entry[K, V]{Key: k, Value: v, ExpiresAt: expiresAt}, at)
}

// Remove removes e from its list, decrements l.len
func (l *LRUList[K, V]) Remove(e *Entry[K, V]) V {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.next = nil // avoid memory leaks
	e.prev = nil // avoid memory leaks
	e.list = nil
	l.len--
	return e.Value
}

// move moves e to next to at.
func (l *LRUList[K, V]) move(e, at *Entry[K, V]) {
	if e == at {
		return
	}
	e.prev.next = e.next
	e.next.prev = e.prev

	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e
}

// PushToFront inserts a new element e with value v at the front of list l and returns e.
func (l *LRUList[K, V]) PushToFront(k K, v V) *Entry[K, V] {
	l.lazyInit()
	return l.insertValue(k, v, time.Time{}, &l.root)
}

// PushToFrontExpirable inserts a new expirable element e with value v at the front of list l and returns e.
func (l *LRUList[K, V]) PushToFrontExpirable(k K, v V, expiresAt time.Time) *Entry[K, V] {
	l.lazyInit()
	return l.insertValue(k, v, expiresAt, &l.root)
}

// MoveToFront moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
func (l *LRUList[K, V]) MoveToFront(e *Entry[K, V]) {
	if e.list != l || l.root.next == e {
		return
	}
	l.move(e, &l.root)
}
