package worm

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"net"
	"strings"
)

type Server struct {
	Addr      string
	Mode      string
	Commands  map[string]func(*Client, []*Value) error
	tlsConfig *tls.Config
	s         net.Listener
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

func newServerWithMode(mode, addr string, tlsConfig *tls.Config) (*Server, error) {
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
		Commands:  map[string]func(*Client, []*Value) error{},
	}, nil
}

func NewTCPServer(addr string, tlsConfig *tls.Config) (*Server, error) {
	return newServerWithMode("tcp", addr, tlsConfig)
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
