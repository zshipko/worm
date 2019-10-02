package worm

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"strconv"
	"strings"
)

var (
	ErrInvalidType        = errors.New("invalid type")
	ErrNotEnoughArguments = errors.New("not enough arguments")
)

type Client struct {
	Version string
	conn    net.Conn
	Input   *bufio.Reader
	Output  *bufio.Writer
	User    *User
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}

	return c.conn.Close()
}

func (c *Client) readCRLF() error {
	_, err := c.Input.ReadByte()
	if err != nil {
		return err
	}

	_, err = c.Input.ReadByte()
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) readLine() (string, error) {
	s, err := c.Input.ReadString('\r')
	if err != nil {
		return "", err
	}

	_, err = c.Input.ReadByte()
	if err != nil {
		return "", err
	}

	return s[:len(s)-1], nil
}

func (c *Client) readLineInt() (int, error) {
	line, err := c.readLine()
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(line)
}

func (c *Client) readBulkString() (*Value, error) {
	length, err := c.readLineInt()
	if err != nil {
		return &NilValue, err
	}

	buf := make([]byte, length)

	_, err = io.ReadFull(c.Input, buf)
	if err != nil {
		return &NilValue, err
	}

	return NewString(string(buf)), c.readCRLF()
}

func (c *Client) readArray() (*Value, error) {
	length, err := c.readLineInt()
	if err != nil {
		return &NilValue, err
	}

	array := make([]*Value, length)

	for i := 0; i < length; i++ {
		array[i], err = c.ReadValue()
		if err != nil {
			return &NilValue, err
		}
	}
	return NewArray(array), nil
}

func (c *Client) readMap() (*Value, error) {
	length, err := c.readLineInt()
	if err != nil {
		return &NilValue, err
	}

	dest := make(map[string]*Value, length)

	for i := 0; i < length; i++ {
		k, err := c.ReadValue()
		if err != nil {
			return &NilValue, err
		}
		v, err := c.ReadValue()
		if err != nil {
			return &NilValue, err
		}

		if !k.Is(String) {
			continue
		}

		dest[k.ToString()] = v
	}

	return NewMap(dest), nil
}

func (c *Client) Read() (*Message, error) {
	ch, err := c.Input.ReadByte()

	message := &Message{Kind: Default}
	switch ch {
	case '_':
		err = c.readCRLF()
		if err != nil {
			return nil, err
		}
		message.Value = &NilValue
	case '$':
		r, _, err := c.Input.ReadRune()
		if err != nil {
			return nil, err
		}

		// Streaming
		if r == '?' {
			return nil, errors.New("Streaming strings are not implemented")
		}

		if err := c.Input.UnreadRune(); err != nil {
			return nil, err
		}

		message.Value, err = c.readBulkString()
		if err != nil {
			return nil, err
		}
	case '=':
		length, err := c.readLineInt()
		if err != nil {
			return nil, err
		}

		buf := make([]byte, length)

		_, err = io.ReadFull(c.Input, buf)
		if err != nil {
			return nil, err
		}

		err = c.readCRLF()
		if err != nil {
			return nil, err
		}

		message.Kind = Verbatim
		message.Type = string(buf[:3])
		message.Value.Kind = Bytes
		message.Value.Data = buf[4:]
	case '+':
		s, err := c.readLine()
		if err != nil {
			return nil, err
		}
		message.Value = NewString(s)
	case '!':
		message.Value, err = c.readBulkString()
		if err != nil {
			return nil, err
		}
		message.Value.Kind = Error
	case '-':
		s, err := c.readLine()
		if err != nil {
			return nil, err
		}
		message.Value = NewError(s)
	case ':':
		s, err := c.readLine()
		if err != nil {
			return nil, err
		}

		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		message.Value = NewInt64(i)
	case ',':
		s, err := c.readLine()
		if err != nil {
			return nil, err
		}

		i, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, err
		}
		message.Value = NewFloat64(i)
	case '(':
		s, err := c.readLine()
		if err != nil {
			return nil, err
		}

		i := big.NewInt(0)
		i, ok := i.SetString(s, 10)
		if !ok {
			return nil, errors.New("Invalid BigInt")
		}

		message.Value = NewBigInt(i)
	case '#':
		b, err := c.readLine()
		if err != nil {
			return nil, err
		}
		v := false
		if b == "t" {
			v = true
		}
		message.Value = NewBool(v)
	case '*':
		r, _, err := c.Input.ReadRune()
		if err != nil {
			return nil, err
		}
		// Streaming
		if r == '?' {
			return nil, errors.New("Streaming strings are not implemented")
		}

		if err := c.Input.UnreadRune(); err != nil {
			return nil, err
		}

		message.Value, err = c.readArray()
		if err != nil {
			return nil, err
		}
	case '~':
		message.Value, err = c.readArray()
		if err != nil {
			return nil, err
		}
		message.Kind = SetReply
	case '>':
		message.Value, err = c.readArray()
		if err != nil {
			return nil, err
		}

		arr := message.Value.ToArray()
		message.Type = arr[0].ToString()
		message.Value = NewArray(arr[1:])
		message.Kind = Push
	case '%':
		message.Value, err = c.readMap()
		if err != nil {
			return nil, err
		}
	case 'p':
		fallthrough
	case 'P':
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}

		parts := strings.Split(line, " ")

		if strings.ToLower(parts[0]) != "ing" {
			return nil, ErrInvalidType
		}

		if len(parts) == 1 {
			parts = append(parts, "PONG")
		}

		message.Value = NewArray([]*Value{
			NewString("PING"),
			NewString(parts[1]),
		})
	case 0:
		return nil, io.EOF
	default:
		log.Println("Invalid message type:", ch)
		return nil, ErrInvalidType
	}

	return message, nil
}

func (c *Client) ReadValue() (*Value, error) {
	msg, err := c.Read()
	if err != nil {
		return &NilValue, err
	}
	return msg.Value, nil
}

func (c *Client) WriteCRLF() error {
	_, err := c.Output.WriteString("\r\n")
	return err
}

func (c *Client) WriteArrayHeader(n int) error {
	_, err := c.Output.Write([]byte(fmt.Sprintf("*%d\r\n", n)))
	return err
}

func (c *Client) WriteMapHeader(n int) error {
	_, err := c.Output.Write([]byte(fmt.Sprintf("%%%d\r\n", n)))
	return err
}

func (c *Client) Write(message *Message) error {
	if message == nil {
		return nil
	}

	if c.Version == "2" {
		return c.writeValueV2(message.Value)
	}

	switch message.Kind {
	case Default:
		c.WriteValue(message.Value)
	case Hello:
		if message.User != nil {
			c.WriteArrayHeader(5)
		} else {
			c.WriteArrayHeader(2)
		}

		c.WriteValue(NewString("HELLO"))
		c.WriteValue(NewString("3"))

		if message.User != nil {
			c.WriteValue(NewString("AUTH"))
			c.WriteValue(NewString(message.User.Name))
			return c.WriteValue(NewString(message.User.Password))
		}

		return nil
	case SetReply:
		a := message.Value.ToArray()
		_, err := c.Output.WriteString(fmt.Sprint("~", len(a), "\r\n"))
		if err != nil {
			return err
		}

		for _, v := range a {
			err = c.WriteValue(v)
			if err != nil {
				return err
			}
		}
	case Push:
		a := message.Value.ToArray()
		_, err := c.Output.WriteString(fmt.Sprint(">", len(a)+1, "\r\n"))
		if err != nil {
			return err
		}

		err = c.WriteValue(NewString(message.Type))
		if err != nil {
			return nil
		}

		for _, v := range a {
			err = c.WriteValue(v)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Client) writeValueV2(val *Value) error {
	var err error

	if val == nil {
		_, err = c.Output.WriteString("$-1\r\n")
		return err
	}

	switch val.Kind {
	case Nil:
		_, err = c.Output.WriteString("$-1\r\n")
	case Bool:
		b := val.ToBool()
		err = c.writeValueV2(NewString(fmt.Sprint(b)))
	case Int64:
		_, err = c.Output.WriteString(fmt.Sprint(":", val.ToInt64(), "\r\n"))
	case Float64:
		err = c.writeValueV2(NewString(fmt.Sprint(val.ToFloat64())))
	case BigInt:
		err = c.writeValueV2(NewString(fmt.Sprint(val.ToBigInt())))
	case String:
		s := val.ToString()
		err := c.WriteStringHeader(len(s))
		if err != nil {
			return err
		}
		if _, err = c.Output.WriteString(s); err != nil {
			return err
		}
		err = c.WriteCRLF()
	case Error:
		s := val.ToError().Error()
		_, err = c.Output.WriteString(fmt.Sprint("-", s, "\r\n"))
	case Bytes:
		s := val.ToBytes()
		err := c.WriteStringHeader(len(s))
		if err != nil {
			return err
		}
		if _, err = c.Output.Write(s); err != nil {
			return err
		}
		err = c.WriteCRLF()
	case Array:
		a := val.ToArray()
		err := c.WriteArrayHeader(len(a))
		if err != nil {
			return err
		}

		for _, v := range a {
			err = c.writeValueV2(v)
			if err != nil {
				return err
			}
		}
	case Map:
		a := val.ToMap()
		err := c.WriteArrayHeader(len(a) * 2)
		if err != nil {
			return err
		}

		for k, v := range a {
			err = c.writeValueV2(New(k))
			if err != nil {
				return err
			}

			err = c.writeValueV2(v)
			if err != nil {
				return err
			}
		}
	}

	return err
}

func (c *Client) WriteValue(val *Value) error {
	var err error

	if c.Version == "2" {
		return c.writeValueV2(val)
	}

	if val == nil {
		_, err = c.Output.WriteString("_\r\n")
		return err
	}

	switch val.Kind {
	case Nil:
		_, err = c.Output.WriteString("_\r\n")
	case Bool:
		b := val.ToBool()
		if b {
			_, err = c.Output.WriteString("#t\r\n")
		} else {
			_, err = c.Output.WriteString("#f\r\n")
		}
	case Int64:
		_, err = c.Output.WriteString(fmt.Sprint(":", val.ToInt64(), "\r\n"))
	case Float64:
		_, err = c.Output.WriteString(fmt.Sprint(",", val.ToFloat64(), "\r\n"))
	case BigInt:
		_, err = c.Output.WriteString(fmt.Sprint("(", val.ToBigInt(), "\r\n"))
	case String:
		s := val.ToString()
		err := c.WriteStringHeader(len(s))
		if err != nil {
			return err
		}
		if _, err = c.Output.WriteString(s); err != nil {
			return err
		}
		err = c.WriteCRLF()
	case Error:
		s := val.ToError().Error()
		if strings.ContainsAny(s, "\r\n") {
			_, err = c.Output.WriteString(fmt.Sprint("!", len(s), "\r\n"))
			if err != nil {
				return err
			}

			if _, err = c.Output.WriteString(s); err != nil {
				return err
			}

			err = c.WriteCRLF()
		} else {
			_, err = c.Output.WriteString(fmt.Sprint("-", s, "\r\n"))
		}
	case Bytes:
		s := val.ToBytes()
		_, err = c.Output.WriteString(fmt.Sprint("=", len(s), "\r\nraw:"))
		if err != nil {
			return err
		}

		if _, err = c.Output.Write(s); err != nil {
			return err
		}

		err = c.WriteCRLF()
	case Array:
		a := val.ToArray()
		err := c.WriteArrayHeader(len(a))
		if err != nil {
			return err
		}

		for _, v := range a {
			err = c.WriteValue(v)
			if err != nil {
				return err
			}
		}
	case Map:
		d := val.ToMap()
		err := c.WriteMapHeader(len(d))
		if err != nil {
			return err
		}

		for k, v := range d {
			err = c.WriteValue(NewString(k))
			if err != nil {
				return err
			}

			err = c.WriteValue(v)
			if err != nil {
				return err
			}
		}
	}

	return err
}

func (c *Client) WriteStringHeader(n int) error {
	_, err := c.Output.WriteString(fmt.Sprint("$", n, "\r\n"))
	return err
}

func (c *Client) WriteSimpleString(s string) error {
	_, err := c.Output.WriteString(fmt.Sprint("+", s, "\r\n"))
	return err
}

func (c *Client) WriteOK() error {
	return c.WriteSimpleString("OK")
}

func (c *Client) WriteError(msg string) error {
	return c.WriteValue(NewError(msg))
}

func (c *Client) Command(args ...string) (*Message, error) {
	length := len(args)
	for i := 0; i < length; i++ {
		if err := c.WriteValue(NewString(args[i])); err != nil {
			return nil, err
		}
	}

	return c.Read()
}

func Connect(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	client := &Client{
		Input:  r,
		Output: w,
		conn:   conn,
	}
	return client, err
}
