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
	Users     map[string]string
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
		Users:     map[string]string{},
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

func (s *Server) CheckUser(user *User) bool {
	if len(s.Users) == 0 {
		return true
	}

	if user == nil {
		return false
	}

	return s.Users[user.Name] == user.Password
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
			if len(args) == 1 {
				client.WriteValue(NewError("malformed HELLO command"))
				client.Output.Flush()
				return
			}

			if args[1].ToString() != "3" {
				client.WriteValue(NewValue(Error, "NOPROTO this protocol is not supported"))
				client.Output.Flush()
				return
			}

			if len(args) >= 4 {
				client.User = &User{
					Name:     args[2].ToString(),
					Password: args[3].ToString(),
				}

				if !s.CheckUser(client.User) {
					client.WriteValue(NewError("auth failed"))
					client.Output.Flush()
					return
				}
			}

			client.WriteValue(NewMap(map[string]*Value{
				"server":  NewString("merz"),
				"version": NewInt(Version),
				"proto":   NewInt(3),
			}))
		} else if cmd == "auth" {
			if len(args) == 1 {
				client.WriteValue(NewError("not enough arguments"))
			} else if len(args) == 2 {
				client.User = &User{
					Name:     "default",
					Password: args[1].ToString(),
				}
			} else if len(args) == 3 {
				client.User = &User{
					Name:     args[1].ToString(),
					Password: args[2].ToString(),
				}
			}

			if !s.CheckUser(client.User) {
				client.WriteValue(NewError("auth failed"))
				client.Output.Flush()
				return
			}

			client.WriteOK()
		} else if cmd == "command" {
			if !s.CheckUser(client.User) {
				client.WriteValue(NewError("auth failed"))
				client.Output.Flush()
				return
			}

			arr := []*Value{}

			for k, _ := range s.Commands {
				arr = append(arr, New(k))
			}

			client.WriteValue(New(arr))
		} else {
			f, ok := s.Commands[cmd]
			if !ok {
				client.WriteValue(NewError("invalid command"))
			} else {
				if err = f(&client, args); err != nil {
					client.WriteValue(NewError(err.Error()))
				}
			}
		}

		client.Output.Flush()
	}
}
