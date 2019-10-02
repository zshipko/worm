package worm

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"net"
	"reflect"
	"strings"
)

const Version = 1

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

func extractCommands(ctx interface{}) map[string]Command {
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
		} else if method.Type.NumIn() != 3 {
			continue
		} else if !method.Type.In(1).ConvertibleTo(reflect.TypeOf(&Client{})) {
			continue
		} else if !method.Type.In(2).ConvertibleTo(reflect.TypeOf([]*Value{})) {
			continue
		}

		val := reflect.ValueOf(ctx)
		commands[name] = func(client *Client, args []*Value) error {
			r := method.Func.Call([]reflect.Value{
				val,
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

	return commands
}

func newServerWithMode(mode, addr string, tlsConfig *tls.Config, ctx interface{}) (*Server, error) {
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

	return &Server{
		tlsConfig: tlsConfig,
		Addr:      addr,
		Mode:      mode,
		s:         s,
		Context:   ctx,
		Commands:  extractCommands(ctx),
	}, nil
}

func NewTCPServer(addr string, tlsConfig *tls.Config, ctx interface{}) (*Server, error) {
	return newServerWithMode("tcp", addr, tlsConfig, ctx)
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

		if cmd == "hello" {
			if args[1].ToString() != "3" {
				client.WriteValue(NewValue(Error, "NOPROTO this protocol is not supported"))
				return
			}

			client.WriteValue(NewMap(map[string]*Value{
				"server":  NewString("merz"),
				"version": NewInt(Version),
				"proto":   NewInt(3),
			}))
			continue
		}

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
