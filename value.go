package worm

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"log"
	"math/big"
	"reflect"
)

type Kind int

const (
	Nil Kind = iota
	Bool
	Int64
	Float64
	BigInt
	String
	Bytes
	Error
	Array
	Map
)

type Value struct {
	Kind Kind
	Data interface{}
}

var NilValue = Value{Kind: Nil, Data: nil}

func (v *Value) Encode(w io.Writer) error {
	return gob.NewEncoder(w).Encode(v)
}

func (v *Value) EncodeBytes() ([]byte, error) {
	b := []byte{}
	buf := bytes.NewBuffer(b)
	if err := v.Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Decode(r io.Reader) (*Value, error) {
	value := &NilValue
	err := gob.NewDecoder(r).Decode(value)
	return value, err
}

func DecodeBytes(b []byte) (*Value, error) {
	buf := bytes.NewBuffer(b)
	return Decode(buf)
}

func NewValue(kind Kind, data interface{}) *Value {
	return &Value{
		kind,
		data,
	}
}

func NewBool(b bool) *Value {
	return NewValue(Bool, b)
}

func NewInt64(i int64) *Value {
	return NewValue(Int64, i)
}

func NewInt(i int) *Value {
	return NewInt64(int64(i))
}

func NewFloat64(f float64) *Value {
	return NewValue(Float64, f)
}

func NewBigInt(i *big.Int) *Value {
	return NewValue(BigInt, i)
}

func NewString(s string) *Value {
	return NewValue(String, s)
}

func NewErrorNoPrefix(s string) *Value {
	return NewValue(Error, s)
}

func NewError(s string) *Value {
	return NewValue(Error, "ERR "+s)
}

func NewBytes(b []byte) *Value {
	return NewValue(Bytes, b)
}

func NewArray(a []*Value) *Value {
	return NewValue(Array, a)
}

func NewMap(m map[string]*Value) *Value {
	return NewValue(Map, m)
}

func New(value interface{}) *Value {
	switch a := value.(type) {
	case *Value:
		return a
	case Value:
		return &a
	case *interface{}:
		return New(*a)
	case bool:
		return NewBool(a)
	case big.Int:
		return NewBigInt(&a)
	case int64:
		return NewInt64(a)
	case int:
		return NewInt(a)
	case uint64:
		return NewInt64(int64(a))
	case int32:
		return NewInt64(int64(a))
	case uint32:
		return NewInt64(int64(a))
	case float32:
		return NewFloat64(float64(a))
	case float64:
		return NewFloat64(float64(a))
	case string:
		return NewString(a)
	case []byte:
		return NewBytes(a)
	case []*Value:
		return NewArray(a)
	case []interface{}:
		dest := make([]*Value, len(a))
		for i, v := range a {
			dest[i] = New(v)
		}
		return NewArray(dest)
	case map[string]*Value:
		return NewMap(a)
	case map[string]interface{}:
		dest := map[string]*Value{}
		for k, v := range a {
			dest[k] = New(v)
		}
		return NewMap(dest)
	case error:
		return NewError(a.Error())
	case Message:
		return a.Value
	default:
		val := reflect.ValueOf(a)

		if val.Kind() != reflect.Struct {
			log.Println("Unknown type in call to New", val.Interface())
			return &NilValue
		}

		dest := map[string]*Value{}
		for i := 0; i < val.NumField(); i++ {
			valueField := val.Field(i)
			typeField := val.Type().Field(i)
			k := typeField.Name
			tag := typeField.Tag.Get("worm")
			if tag != "" {
				k = tag
			}

			dest[k] = New(valueField.Interface())
		}

		return NewMap(dest)
	}
}

func (v *Value) Is(kind Kind) bool {
	return v.Kind == kind
}

func (v *Value) ToMap() map[string]*Value {
	if v.Is(Map) {
		return v.Data.(map[string]*Value)
	}

	return nil
}

func (v *Value) ToArray() []*Value {
	if v.Is(Array) {
		return v.Data.([]*Value)
	}

	return nil
}

func (v *Value) ToBytes() []byte {
	if v.Is(Bytes) {
		return v.Data.([]byte)
	}

	return nil
}

func (v *Value) ToString() string {
	if v.Is(String) {
		return v.Data.(string)
	}

	return ""
}

func (v *Value) ToError() error {
	if v.Is(Error) {
		switch a := v.Data.(type) {
		case []byte:
			return errors.New(string(a))
		case string:
			return errors.New(a)
		}
	}

	return nil
}

func (v *Value) ToBigInt() *big.Int {
	if v.Is(BigInt) {
		return v.Data.(*big.Int)
	}

	return big.NewInt(0)
}

func (v *Value) ToInt64() int64 {
	if v.Is(Int64) {
		return v.Data.(int64)
	}

	return int64(0)
}

func (v *Value) ToFloat64() float64 {
	if v.Is(Float64) {
		return v.Data.(float64)
	}

	return float64(0)
}

func (v *Value) ToBool() bool {
	if v.Is(Bool) {
		return v.Data.(bool)
	}

	return false
}

func (v *Value) IsNil() bool {
	return v.Is(Nil)
}
