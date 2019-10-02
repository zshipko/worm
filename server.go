package worm

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net"
	"reflect"
	"strings"
)

type Command = func(*Client, []*Value) error

type Server struct {
	Addr      string
	Mode      string
	Context   interface{}
	Commands  map[string]Command
	tlsConfig *tls.Config
	s         net.Listener
	Closed    bool
}

func LoadX509KeyPair(certFile, keyFile string) (*tls.Config, error) {
	kp, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{kp},
		Rand:         rand.Reader,
	}, nil
}

func (s *Server) Close() error {
	return s.s.Close()
}

func newServerWithMode(ctx interface{}, mode, addr string, tlsConfig *tls.Config) (*Server, error) {
	var s net.Listener
	var err error

	if tlsConfig != nil {
		s, err = tls.Listen(mode, addr, tlsConfig)
	} else {
		s, err = net.Listen(mode, addr)
	}
	if err != nil {
		return nil, err
	}

	commands := map[string]Command{}

	typ := reflect.TypeOf(ctx)
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		name := strings.ToLower(method.Name)

		if method.Type.NumOut() != 1 {
			continue
		}

		if method.Type.Out(0).Name() != "error" {
			continue
		}

		if method.Type.NumIn() != 3 {
			continue
		}

		if !method.Type.In(1).ConvertibleTo(reflect.TypeOf(&Client{})) {
			continue
		}

		if !method.Type.In(2).ConvertibleTo(reflect.TypeOf([]*Value{})) {
			continue
		}

		commands[name] = func(client *Client, args []*Value) error {
			r := method.Func.Call([]reflect.Value{
				reflect.ValueOf(ctx),
				reflect.ValueOf(client),
				reflect.ValueOf(args),
			})[0].Interface()
			switch e := r.(type) {
			case nil:
				return nil
			case error:
				return e
			default:
				panic("Invalid return type")
			}
		}
	}

	return &Server{
		tlsConfig: tlsConfig,
		Addr:      addr,
		Mode:      mode,
		s:         s,
		Context:   ctx,
		Commands:  commands,
	}, nil
}

func NewTCPServer(ctx interface{}, addr string, tlsConfig *tls.Config) (*Server, error) {
	return newServerWithMode(ctx, "tcp", addr, tlsConfig)
}

func (server *Server) Run() error {
	for {
		conn, err := server.s.Accept()
		if err != nil {
			return err
		}

		r := bufio.NewReader(conn)
		w := bufio.NewWriter(conn)

		client := Client{
			Input:  r,
			Output: w,
			conn:   conn,
		}

		go server.handleClient(client)
	}
}

func (s *Server) handleClient(client Client) {
	defer client.Close()

	for {
		msg, err := client.Read()
		if err != nil {
			return
		}

		args := msg.Value.ToArray()
		cmd := strings.ToLower(args[0].ToString())

		f, ok := s.Commands[cmd]
		if !ok {
			client.WriteValue(NewError("Invalid command"))
			client.Output.Flush()
			continue
		}

		if err = f(&client, args); err != nil {
			client.WriteValue(NewError(err.Error()))
		}

		client.Output.Flush()
	}
}
