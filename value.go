package worm

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"reflect"
	"strconv"
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

func NewNil() *Value {
	return NewValue(Nil, nil)
}

func New(value interface{}) *Value {
	if value == nil {
		return NewNil()
	}

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
			log.Printf("Unknown type in call to New %T\n", val)
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
	} else if v.Is(Array) {
		arr := v.Data.([]*Value)
		if len(arr)%2 != 0 {
			return nil
		}

		dest := map[string]*Value{}

		for i := 0; i < len(arr); i += 2 {
			dest[arr[i].ToString()] = arr[i+1]
		}

		return dest
	}

	return nil
}

func (v *Value) ToArray() []*Value {
	if v.Is(Array) {
		return v.Data.([]*Value)
	} else if v.Is(Map) {
		dest := []*Value{}

		for k, v := range v.Data.(map[string]*Value) {
			dest = append(dest, NewString(k), v)
		}

		return dest
	}

	return nil
}

func (v *Value) ToBytes() []byte {
	if v.Is(Bytes) {
		return v.Data.([]byte)
	} else if v.Is(String) {
		return []byte(v.Data.(string))
	}

	return nil
}

func (v *Value) ToString() string {
	if v.Is(String) {
		return v.Data.(string)
	} else if v.Is(Int64) || v.Is(Float64) || v.Is(BigInt) || v.Is(Bool) {
		return fmt.Sprint(v.Data)
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
	} else if v.Is(Int64) {
		return big.NewInt(v.Data.(int64))
	}

	return big.NewInt(0)
}

func (v *Value) ToInt64() int64 {
	if v.Is(Int64) {
		return v.Data.(int64)
	} else if v.Is(Float64) {
		return int64(v.Data.(float64))
	} else if v.Is(String) {
		n, err := strconv.ParseInt(v.Data.(string), 10, 64)
		if err == nil {
			return n
		}
	}

	return int64(0)
}

func (v *Value) ToInt() int {
	return int(v.ToInt64())
}

func (v *Value) ToFloat64() float64 {
	if v.Is(Float64) {
		return floatv.Data.(float64)
	} else if v.Is(Int64) {
		return float64(v.Data.(int64))
	} else if v.Is(String) {
		n, err := strconv.ParseFloat(v.Data.(string), 64)
		if err == nil {
			return n
		}
	}

	return float64(0)
}

func (v *Value) ToFloat32() float32 {
	if v.Is(Float64) {
		return float32(v.Data.(float64))
	} else if v.Is(Int64) {
		return float32(v.Data.(int64))
	} else if v.Is(String) {
		n, err := strconv.ParseFloat(v.Data.(string), 32)
		if err == nil {
			return float32(n)
		}
	}

	return float32(0)
}

func (v *Value) ToBool() bool {
	if v.Is(Bool) {
		return v.Data.(bool)
	} else if v.Is(Int64) {
		return v.Data.(int64) != 0
	}

	return false
}

func (v *Value) IsNil() bool {
	return v.Is(Nil)
}

func (v *Value) UnmarshalJSON(b []byte) error {
	var s interface{}
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	v.Data = New(s)
	return nil
}

func (v Value) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Data)
}
