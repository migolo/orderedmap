package orderedmap

import (
	"bytes"
	"encoding/json"
	"sort"
)

type Pair[T any] struct {
	key   string
	value T
}

func (kv *Pair[T]) Key() string {
	return kv.key
}

func (kv *Pair[T]) Value() interface{} {
	return kv.value
}

type ByPair[T any] struct {
	Pairs    []*Pair[T]
	LessFunc func(a *Pair[T], j *Pair[T]) bool
}

func (a ByPair[T]) Len() int           { return len(a.Pairs) }
func (a ByPair[T]) Swap(i, j int)      { a.Pairs[i], a.Pairs[j] = a.Pairs[j], a.Pairs[i] }
func (a ByPair[T]) Less(i, j int) bool { return a.LessFunc(a.Pairs[i], a.Pairs[j]) }

type OrderedMap[T any] struct {
	keys       []string
	values     map[string]T
	escapeHTML bool
}

func New[T any]() *OrderedMap[T] {
	o := OrderedMap[T]{}
	o.keys = []string{}
	o.values = map[string]T{}
	o.escapeHTML = true
	return &o
}

func (o *OrderedMap[T]) SetEscapeHTML(on bool) {
	o.escapeHTML = on
}

func (o *OrderedMap[T]) Get(key string) (T, bool) {
	val, exists := o.values[key]
	return val, exists
}

func (o *OrderedMap[T]) Set(key string, value T) {
	_, exists := o.values[key]
	if !exists {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

func (o *OrderedMap[T]) Delete(key string) {
	// check key is in use
	_, ok := o.values[key]
	if !ok {
		return
	}
	// remove from keys
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
	// remove from values
	delete(o.values, key)
}

func (o *OrderedMap[T]) Keys() []string {
	return o.keys
}

// SortKeys Sort the map keys using your sort func
func (o *OrderedMap[T]) SortKeys(sortFunc func(keys []string)) {
	sortFunc(o.keys)
}

// Sort Sort the map using your sort func
func (o *OrderedMap[T]) Sort(lessFunc func(a *Pair[T], b *Pair[T]) bool) {
	pairs := make([]*Pair[T], len(o.keys))
	for i, key := range o.keys {
		pairs[i] = &Pair[T]{key, o.values[key]}
	}

	sort.Sort(ByPair[T]{pairs, lessFunc})

	for i, pair := range pairs {
		o.keys[i] = pair.key
	}
}

func (o *OrderedMap[T]) UnmarshalJSON(b []byte) error {
	if o.values == nil {
		o.values = map[string]T{}
	}
	err := json.Unmarshal(b, &o.values)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if _, err = dec.Token(); err != nil { // skip '{'
		return err
	}
	o.keys = make([]string, 0, len(o.values))
	return decodeOrderedMap(dec, o)
}

func decodeOrderedMap[T any](dec *json.Decoder, o *OrderedMap[T]) error {
	hasKey := make(map[string]bool, len(o.values))
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		if delim, ok := token.(json.Delim); ok && delim == '}' {
			return nil
		}
		key := token.(string)
		if hasKey[key] {
			// duplicate key
			for j, k := range o.keys {
				if k == key {
					copy(o.keys[j:], o.keys[j+1:])
					break
				}
			}
			o.keys[len(o.keys)-1] = key
		} else {
			hasKey[key] = true
			o.keys = append(o.keys, key)
		}

		token, err = dec.Token()
		if err != nil {
			return err
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				if err = decodeOrderedMap(dec, &OrderedMap[T]{}); err != nil {
					return err
				}
			case '[':
				if err = decodeSlice(dec, []T{}, o.escapeHTML); err != nil {
					return err
				}
			}
		}
	}
}

func decodeSlice[T any](dec *json.Decoder, s []T, escapeHTML bool) error {
	for index := 0; ; index++ {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				if index < len(s) {
					if err = decodeOrderedMap(dec, &OrderedMap[T]{}); err != nil {
						return err
					}
				} else if err = decodeOrderedMap(dec, &OrderedMap[T]{}); err != nil {
					return err
				}
			case '[':
				if err = decodeSlice(dec, []T{}, escapeHTML); err != nil {
					return err
				}
			case ']':
				return nil
			}
		}
	}
}

func (o OrderedMap[T]) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(o.escapeHTML)
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		// add key
		if err := encoder.Encode(k); err != nil {
			return nil, err
		}
		buf.WriteByte(':')
		// add value
		if err := encoder.Encode(o.values[k]); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
