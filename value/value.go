package value

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type Type string

const (
	TypeNil      Type = "nil"
	TypeBool     Type = "boolean"
	TypeNumber   Type = "number"
	TypeString   Type = "string"
	TypeTable    Type = "table"
	TypeFunction Type = "function"
	TypeUserData Type = "userdata"
	TypeThread   Type = "thread"
)

type Value interface {
	Type() Type
	String() string
}

type NilType struct{}

func (NilType) Type() Type     { return TypeNil }
func (NilType) String() string { return "nil" }

var Nil Value = NilType{}

type Bool bool

func (b Bool) Type() Type { return TypeBool }
func (b Bool) String() string {
	if b {
		return "true"
	}
	return "false"
}

type Number float64

func (n Number) Type() Type { return TypeNumber }
func (n Number) String() string {
	f := float64(n)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Sprint(f)
	}
	if f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

type String string

func (s String) Type() Type     { return TypeString }
func (s String) String() string { return string(s) }

type UserData struct {
	Data      any
	metatable *Table
}

func NewUserData(data any) *UserData {
	return &UserData{Data: data}
}

func (u *UserData) Type() Type { return TypeUserData }
func (u *UserData) String() string {
	if u == nil {
		return "nil"
	}
	return fmt.Sprintf("userdata: %p", u)
}

func (u *UserData) Metatable() *Table { return u.metatable }

func (u *UserData) SetMetatable(mt *Table) { u.metatable = mt }

type Table struct {
	array     []Value
	dict      map[Value]Value
	metatable *Table
}

func NewTable() *Table {
	return &Table{dict: map[Value]Value{}}
}

func (t *Table) Type() Type { return TypeTable }
func (t *Table) String() string {
	if t == nil {
		return "nil"
	}
	return fmt.Sprintf("table: %p", t)
}

func (t *Table) Append(v Value) {
	if v == nil {
		v = Nil
	}
	t.RawSet(Number(t.Len()+1), v)
}

func (t *Table) Insert(pos int, v Value) {
	if v == nil {
		v = Nil
	}
	limit := t.Len() + 1
	if pos <= 0 || pos > limit {
		pos = limit
	}
	i := pos - 1
	if len(t.array) > limit-1 {
		t.array = t.array[:limit-1]
	}
	t.array = append(t.array, Nil)
	copy(t.array[i+1:], t.array[i:])
	t.array[i] = v
}

func (t *Table) Set(key Value, v Value) {
	if t.RawGet(key) != Nil {
		t.RawSet(key, v)
		return
	}
	if t.metatable != nil {
		if target, ok := t.metatable.RawGet(String("__newindex")).(*Table); ok {
			target.Set(key, v)
			return
		}
	}
	t.RawSet(key, v)
}

func (t *Table) RawSet(key Value, v Value) {
	if key == nil {
		panic("table index is nil")
	}
	if _, ok := key.(NilType); ok {
		panic("table index is nil")
	}
	if n, ok := key.(Number); ok && math.IsNaN(float64(n)) {
		panic("table index is NaN")
	}
	if v == nil {
		v = Nil
	}
	if n, ok := key.(Number); ok && float64(n) >= 1 && float64(n) == math.Trunc(float64(n)) {
		i := int(n) - 1
		if _, ok := v.(NilType); ok {
			if i >= 0 && i < len(t.array) {
				t.array[i] = Nil
			}
			return
		}
		for len(t.array) <= i {
			t.array = append(t.array, Nil)
		}
		t.array[i] = v
		return
	}
	if _, ok := v.(NilType); ok {
		delete(t.dict, key)
		return
	}
	t.dict[key] = v
}

func (t *Table) Get(key Value) Value {
	if v := t.RawGet(key); v != Nil {
		return v
	}
	if t.metatable != nil {
		index := t.metatable.RawGet(String("__index"))
		switch idx := index.(type) {
		case *Table:
			return idx.Get(key)
		case interface{ CallMeta(Value) Value }:
			return idx.CallMeta(key)
		}
	}
	return Nil
}

func (t *Table) RawGet(key Value) Value {
	if n, ok := key.(Number); ok && float64(n) >= 1 && float64(n) == math.Trunc(float64(n)) {
		i := int(n) - 1
		if i >= 0 && i < len(t.array) {
			return t.array[i]
		}
		return Nil
	}
	if v, ok := t.dict[key]; ok {
		return v
	}
	return Nil
}

func (t *Table) Len() int {
	for i := len(t.array) - 1; i >= 0; i-- {
		if t.array[i] != Nil {
			return i + 1
		}
	}
	return 0
}

func (t *Table) MaxN() int {
	for i := len(t.array) - 1; i >= 0; i-- {
		if t.array[i] != Nil {
			return i + 1
		}
	}
	return 0
}

func (t *Table) Metatable() *Table { return t.metatable }

func (t *Table) SetMetatable(mt *Table) { t.metatable = mt }

func (t *Table) Remove(pos int) Value {
	if pos <= 0 {
		pos = t.Len()
	}
	i := pos - 1
	if i < 0 || i >= len(t.array) {
		return Nil
	}
	v := t.array[i]
	t.array = append(t.array[:i], t.array[i+1:]...)
	for len(t.array) > 0 && t.array[len(t.array)-1] == Nil {
		t.array = t.array[:len(t.array)-1]
	}
	return v
}

func (t *Table) Sort() {
	sort.SliceStable(t.array, func(i, j int) bool {
		left, leftOK := ToNumber(t.array[i])
		right, rightOK := ToNumber(t.array[j])
		if leftOK && rightOK {
			return left < right
		}
		return t.array[i].String() < t.array[j].String()
	})
}

func (t *Table) SortFunc(less func(a, b Value) bool) {
	sort.SliceStable(t.array, func(i, j int) bool {
		return less(t.array[i], t.array[j])
	})
}

func (t *Table) Concat(sep string, start, end int) string {
	if start > end {
		return ""
	}
	parts := make([]string, 0, end-start+1)
	for i := start; i <= end; i++ {
		parts = append(parts, t.RawGet(Number(i)).String())
	}
	return strings.Join(parts, sep)
}

func (t *Table) Values(start, end int) []Value {
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > len(t.array) {
		end = len(t.array)
	}
	if start > end {
		return nil
	}
	out := make([]Value, 0, end-start+1)
	for i := start - 1; i < end; i++ {
		out = append(out, t.array[i])
	}
	return out
}

func (t *Table) Keys() []Value {
	keys := make([]Value, 0, len(t.dict))
	for key := range t.dict {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i].Type() != keys[j].Type() {
			return keys[i].Type() < keys[j].Type()
		}
		return keys[i].String() < keys[j].String()
	})
	return keys
}

func (t *Table) Entries() [][2]Value {
	entries := make([][2]Value, 0, len(t.array)+len(t.dict))
	for i, v := range t.array {
		if v == Nil {
			continue
		}
		entries = append(entries, [2]Value{Number(i + 1), v})
	}
	for _, key := range t.Keys() {
		if t.dict[key] == Nil {
			continue
		}
		entries = append(entries, [2]Value{key, t.dict[key]})
	}
	return entries
}

func IsTruthy(v Value) bool {
	if v == nil {
		return false
	}
	switch x := v.(type) {
	case NilType:
		return false
	case Bool:
		return bool(x)
	default:
		return true
	}
}

func Equal(a, b Value) bool {
	if a == nil {
		a = Nil
	}
	if b == nil {
		b = Nil
	}
	switch av := a.(type) {
	case NilType:
		_, ok := b.(NilType)
		return ok
	case Bool:
		bv, ok := b.(Bool)
		return ok && av == bv
	case Number:
		bv, ok := b.(Number)
		return ok && av == bv
	case String:
		bv, ok := b.(String)
		return ok && av == bv
	default:
		return a == b
	}
}

func ToNumber(v Value) (float64, bool) {
	switch x := v.(type) {
	case Number:
		return float64(x), true
	case String:
		n, err := strconv.ParseFloat(strings.TrimSpace(string(x)), 64)
		return n, err == nil
	default:
		return 0, false
	}
}
