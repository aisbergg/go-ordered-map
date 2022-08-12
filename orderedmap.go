// Package orderedmap implements an ordered map, i.e. a map that also keeps track of
// the order in which keys were inserted.
//
// All operations are constant-time.
//
// Github repo: https://github.com/wk8/go-ordered-map
//
package orderedmap

import (
	"bytes"
	"container/list"
	"encoding/json"
	"fmt"
	"reflect"
)

type Pair struct {
	Key   interface{}
	Value interface{}

	element *list.Element
}

type OrderedMap struct {
	pairs map[interface{}]*Pair
	list  *list.List
}

// New creates a new OrderedMap.
func New() *OrderedMap {
	return &OrderedMap{
		pairs: make(map[interface{}]*Pair),
		list:  list.New(),
	}
}

// NewWithHint creates a new OrderedMap with the given size hint.
func NewWithHint(hint int) *OrderedMap {
	return &OrderedMap{
		pairs: make(map[interface{}]*Pair, hint),
		list:  list.New(),
	}
}

// NewWithPairs creates a new OrderedMap and adds the given key-value pairs.
func NewWithPairs(keyValuePairs ...interface{}) *OrderedMap {
	if len(keyValuePairs)%2 != 0 {
		panic("must provide an even number of key-value pairs")
	}
	om := &OrderedMap{
		pairs: make(map[interface{}]*Pair, len(keyValuePairs)/2),
		list:  list.New(),
	}
	for i := 0; i < len(keyValuePairs); i += 2 {
		key := keyValuePairs[i]
		value := keyValuePairs[i+1]
		om.Set(key, value)
	}
	return om
}

// NewWithMap creates a new OrderedMap from the given map.
func NewWithMap(m interface{}) *OrderedMap {
	mVal := reflect.ValueOf(m)
	if mVal.Kind() != reflect.Map {
		panic("must provide a map")
	}
	om := &OrderedMap{
		pairs: make(map[interface{}]*Pair, mVal.Len()),
		list:  list.New(),
	}
	for _, key := range mVal.MapKeys() {
		om.Set(key.Interface(), mVal.MapIndex(key).Interface())
	}
	return om
}

// Get looks for the given key, and returns the value associated with it,
// or nil if not found. The boolean it returns says whether the key is present in the map.
func (om *OrderedMap) Get(key interface{}) (interface{}, bool) {
	if pair, present := om.pairs[key]; present {
		return pair.Value, present
	}
	return nil, false
}

// Load is an alias for Get, mostly to present an API similar to `sync.Map`'s.
func (om *OrderedMap) Load(key interface{}) (interface{}, bool) {
	return om.Get(key)
}

// GetPair looks for the given key, and returns the pair associated with it,
// or nil if not found. The Pair struct can then be used to iterate over the ordered map
// from that point, either forward or backward.
func (om *OrderedMap) GetPair(key interface{}) *Pair {
	return om.pairs[key]
}

// Set sets the key-value pair, and returns what `Get` would have returned
// on that key prior to the call to `Set`.
func (om *OrderedMap) Set(key interface{}, value interface{}) (interface{}, bool) {
	if pair, present := om.pairs[key]; present {
		oldValue := pair.Value
		pair.Value = value
		return oldValue, true
	}

	pair := &Pair{
		Key:   key,
		Value: value,
	}
	pair.element = om.list.PushBack(pair)
	om.pairs[key] = pair

	return nil, false
}

// Store is an alias for Set, mostly to present an API similar to `sync.Map`'s.
func (om *OrderedMap) Store(key interface{}, value interface{}) (interface{}, bool) {
	return om.Set(key, value)
}

// Delete removes the key-value pair, and returns what `Get` would have returned
// on that key prior to the call to `Delete`.
func (om *OrderedMap) Delete(key interface{}) (interface{}, bool) {
	if pair, present := om.pairs[key]; present {
		om.list.Remove(pair.element)
		delete(om.pairs, key)
		return pair.Value, true
	}
	return nil, false
}

// Len returns the length of the ordered map.
func (om *OrderedMap) Len() int {
	return len(om.pairs)
}

// Oldest returns a pointer to the oldest pair. It's meant to be used to iterate on the ordered map's
// pairs from the oldest to the newest, e.g.:
// for pair := orderedMap.Oldest(); pair != nil; pair = pair.Next() { fmt.Printf("%v => %v\n", pair.Key, pair.Value) }
func (om *OrderedMap) Oldest() *Pair {
	return listElementToPair(om.list.Front())
}

// Newest returns a pointer to the newest pair. It's meant to be used to iterate on the ordered map's
// pairs from the newest to the oldest, e.g.:
// for pair := orderedMap.Oldest(); pair != nil; pair = pair.Next() { fmt.Printf("%v => %v\n", pair.Key, pair.Value) }
func (om *OrderedMap) Newest() *Pair {
	return listElementToPair(om.list.Back())
}

// Range calls f sequentially for each key and value present in the map. If f returns false, Range stops the iteration.
func (om *OrderedMap) Range(f func(key, value interface{}) bool) {
	for pair := listElementToPair(om.list.Front()); pair != nil; pair = pair.Next() {
		if cont := f(pair.Key, pair.Value); !cont {
			break
		}
	}
}

// RangeReverse works like Range, but in reverse order.
func (om *OrderedMap) RangeReverse(f func(key, value interface{}) bool) {
	for pair := listElementToPair(om.list.Back()); pair != nil; pair = pair.Prev() {
		if cont := f(pair.Key, pair.Value); !cont {
			break
		}
	}
}

// Next returns a pointer to the next pair.
func (p *Pair) Next() *Pair {
	return listElementToPair(p.element.Next())
}

// Prev returns a pointer to the previous pair.
func (p *Pair) Prev() *Pair {
	return listElementToPair(p.element.Prev())
}

func listElementToPair(element *list.Element) *Pair {
	if element == nil {
		return nil
	}
	return element.Value.(*Pair)
}

// KeyNotFoundError may be returned by functions in this package when they're called with keys that are not present
// in the map.
type KeyNotFoundError struct {
	MissingKey interface{}
}

var _ error = &KeyNotFoundError{}

func (e *KeyNotFoundError) Error() string {
	return fmt.Sprintf("missing key: %v", e.MissingKey)
}

// MoveAfter moves the value associated with key to its new position after the one associated with markKey.
// Returns an error iff key or markKey are not present in the map.
func (om *OrderedMap) MoveAfter(key, markKey interface{}) error {
	elements, err := om.getElements(key, markKey)
	if err != nil {
		return err
	}
	om.list.MoveAfter(elements[0], elements[1])
	return nil
}

// MoveBefore moves the value associated with key to its new position before the one associated with markKey.
// Returns an error iff key or markKey are not present in the map.
func (om *OrderedMap) MoveBefore(key, markKey interface{}) error {
	elements, err := om.getElements(key, markKey)
	if err != nil {
		return err
	}
	om.list.MoveBefore(elements[0], elements[1])
	return nil
}

func (om *OrderedMap) getElements(keys ...interface{}) ([]*list.Element, error) {
	elements := make([]*list.Element, len(keys))
	for i, k := range keys {
		pair, present := om.pairs[k]
		if !present {
			return nil, &KeyNotFoundError{k}
		}
		elements[i] = pair.element
	}
	return elements, nil
}

// MoveToBack moves the value associated with key to the back of the ordered map.
// Returns an error iff key is not present in the map.
func (om *OrderedMap) MoveToBack(key interface{}) error {
	pair, present := om.pairs[key]
	if !present {
		return &KeyNotFoundError{key}
	}
	om.list.MoveToBack(pair.element)
	return nil
}

// MoveToFront moves the value associated with key to the front of the ordered map.
// Returns an error iff key is not present in the map.
func (om *OrderedMap) MoveToFront(key interface{}) error {
	pair, present := om.pairs[key]
	if !present {
		return &KeyNotFoundError{key}
	}
	om.list.MoveToFront(pair.element)
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (om *OrderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	encoder := json.NewEncoder(&buf)
	i := 0
	for pair := listElementToPair(om.list.Front()); pair != nil; pair = pair.Next() {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encoder.Encode(pair.Key); err != nil {
			return nil, err
		}
		buf.WriteByte(':')
		if err := encoder.Encode(pair.Value); err != nil {
			return nil, err
		}
		i++
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
