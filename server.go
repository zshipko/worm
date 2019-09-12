package worm

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"net"
	"strings"

	"github.com/akrylysov/pogreb"
)

type Command interface {
	Exec(*DB, *Client, []*Value) error
}

type Server struct {
	Addr      string
	Mode      string
	Context   interface{}
	Commands  map[string]Command
	tlsConfig *tls.Config
	s         net.Listener
	DB        DB
	Closed    bool
}

// TODO: make DB an interface
type DB struct {
	Store *pogreb.DB
}

func (db *DB) Put(key string, value *Value) error {
	v, err := value.EncodeBytes()
	if err != nil {
		return err
	}

	return db.Store.Put([]byte(key), v)
}

func (db *DB) Get(key string) (*Value, error) {
	v, err := db.Store.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	if v == nil {
		return &NilValue, nil
	}

	vx, err := DecodeBytes(v)
	if err != nil {
		return nil, err
	}

	return vx, nil
}

func (db *DB) Delete(key string) error {
	return db.Store.Delete([]byte(key))
}

func (db *DB) Close() error {
	return db.Store.Close()
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
	if err := s.DB.Close(); err != nil {
		return err
	}
	return s.s.Close()
}

func newServerWithMode(mode, addr string, tlsConfig *tls.Config) (*Server, error) {
	var s net.Listener
	var err error

	db, err := pogreb.Open("./db", &pogreb.Options{
		BackgroundSyncInterval: -1,
	})
	if err != nil {
		return nil, err
	}

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
		Commands:  map[string]Command{},
		DB:        DB{Store: db},
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

		if err = f.Exec(&s.DB, &client, args); err != nil {
			client.WriteValue(NewError(err.Error()))
		}

		s.DB.Store.Sync()
		client.Output.Flush()
	}
}
