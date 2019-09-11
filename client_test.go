package worm

import (
	"bufio"
	"bytes"
	"reflect"
	"testing"
)

type example struct {
	A int           `worm:"a"`
	B float32       `worm:"b"`
	C []interface{} `worm:"c"`
}

func TestProtocolRoundtrip(t *testing.T) {
	a := New(
		example{
			A: 1,
			B: 2.,
			C: []interface{}{true},
		},
	)

	t.Log(a)

	b := []byte{}
	buf := bytes.NewBuffer(b)

	output := bufio.NewWriter(buf)
	input := bufio.NewReader(buf)

	client := Client{
		Output: output,
		Input:  input,
	}

	if err := client.WriteValue(a); err != nil {
		t.Fatal(err)
	}
	client.Output.Flush()

	x, err := client.ReadValue()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(a, x) {
		t.Fatal("Invalid rountrip value:", a, x)
	}
}
