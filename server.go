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

const WormVersion = 1

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
	val := reflect.ValueOf(ctx)
	clientType := reflect.TypeOf(&Client{})
	valueType := reflect.TypeOf(&Value{})
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		name := strings.ToLower(method.Name)

		if method.Type.NumOut() != 1 {
			continue
		}

		if method.Type.Out(0).Name() != "error" {
			continue
		} else if method.Type.NumIn() >= 2 && method.Type.In(0) == typ && method.Type.In(1) == clientType {
			variadic := method.Type.IsVariadic()

			ok := true
			for i := 2; i < method.Type.NumIn(); i++ {
				if method.Type.In(i) != valueType {
					if i == method.Type.NumIn()-1 && variadic && method.Type.In(i).Elem() == valueType {
						continue
					}
					ok = false
					break
				}
			}
			if !ok {
				continue
			}

			commands[name] = func(client *Client, args []*Value) error {
				if !variadic && len(args) != method.Type.NumIn()-2 {
					return client.WriteError(fmt.Sprintf("invalid argument count, expected %d but got %d", method.Type.NumIn()-2, len(args)))
				}
				vargs := []reflect.Value{val}
				vargs = append(vargs, reflect.ValueOf(client))
				for _, arg := range args {
					vargs = append(vargs, reflect.ValueOf(arg))
				}
				r := method.Func.Call(vargs)[0].Interface()
				return fixReturnValue(r)
			}
		}
	}

	return commands
}

func fixReturnValue(r interface{}) error {
	switch e := r.(type) {
	case nil:
		return nil
	case error:
		return e
	default:
		panic("Invalid return type")
	}
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
			Input:   r,
			Output:  w,
			conn:    conn,
			Version: "2",
			Data:    map[string]interface{}{},
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

func (s *Server) handleHello(client *Client, args []*Value) {
	if len(args) == 0 {
		client.WriteValue(NewError("malformed HELLO command"))
		return
	}

	client.Version = args[0].ToString()

	if client.Version != "2" && client.Version != "3" {
		client.WriteValue(NewValue(Error, "NOPROTO this protocol is not supported"))
		return
	}

	if len(args) >= 3 {
		client.User = &User{
			Name:     args[1].ToString(),
			Password: args[2].ToString(),
		}

		if !s.CheckUser(client.User) {
			client.WriteValue(NewError("auth failed"))
			return
		}
	}

	client.WriteValue(NewMap(map[string]*Value{
		"server":  NewString("merz"),
		"version": NewInt(WormVersion),
		"proto":   NewInt(3),
	}))
}

func (s *Server) handleAuth(client *Client, args []*Value) {
	if len(args) == 0 {
		client.WriteValue(New(ErrNotEnoughArguments))
	} else if len(args) == 1 {
		client.User = &User{
			Name:     "default",
			Password: args[0].ToString(),
		}
	} else if len(args) == 2 {
		client.User = &User{
			Name:     args[0].ToString(),
			Password: args[1].ToString(),
		}
	}

	if !s.CheckUser(client.User) {
		client.WriteValue(NewError("auth failed"))
		return
	}

	client.WriteOK()
}

func (s *Server) listCommands(client *Client) {
	if !s.CheckUser(client.User) {
		client.WriteValue(NewError("auth failed"))
		return
	}

	arr := []*Value{}

	for k, _ := range s.Commands {
		arr = append(arr, New(k))
	}

	client.WriteValue(New(arr))
}

func (s *Server) handleClient(client Client) {
	defer client.Close()

	for {
		msg, err := client.Read()
		if err != nil {
			return
		}

		args := msg.Value.ToArray()
		if len(args) == 0 {
			return
		}

		cmd := strings.ToLower(args[0].ToString())
		args = args[1:]

		f, ok := s.Commands[cmd]
		if ok {
			if err = f(&client, args); err != nil {
				client.WriteValue(NewError(err.Error()))
			}
		} else {
			if cmd == "hello" {
				s.handleHello(&client, args)
			} else if cmd == "auth" {
				s.handleAuth(&client, args)
			} else if cmd == "command" {
				s.listCommands(&client)
			} else if cmd == "ping" {
				if len(args) > 0 {
					client.WriteValue(args[0])
				} else {
					client.WriteValue(New("PONG"))
				}
			} else {
				client.WriteValue(NewError("invalid command"))
			}
		}

		client.Output.Flush()
	}
}
