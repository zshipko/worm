package main

import (
	"log"
	"sync"

	"github.com/zshipko/worm"
)

var lock sync.Mutex

type Context struct {
	db map[string]*worm.Value
}

func (cmd *Context) Get(client *worm.Client, args []*worm.Value) error {
	lock.Lock()
	defer lock.Unlock()

	if len(args) < 2 {
		return worm.ErrNotEnoughArguments
	}

	k := args[1].ToString()
	return client.WriteValue(cmd.db[k])
}

type cmdSet struct {
	db map[string]*worm.Value
}

func (cmd *Context) Set(client *worm.Client, args []*worm.Value) error {
	lock.Lock()
	defer lock.Unlock()

	if len(args) < 3 {
		return worm.ErrNotEnoughArguments
	}

	k := args[1].ToString()
	v := args[2]
	cmd.db[k] = v

	return client.WriteOK()
}

func (cmd *Context) Del(client *worm.Client, args []*worm.Value) error {
	lock.Lock()
	defer lock.Unlock()

	if len(args) < 2 {
		return worm.ErrNotEnoughArguments
	}

	count := 0

	for _, k := range args[1:] {
		delete(cmd.db, k.ToString())
		count += 1
	}

	if count == 1 && len(args) == 2 {
		return client.WriteOK()
	}

	return client.WriteValue(worm.New(int64(count)))
}

func main() {
	// cert, err := LoadX509KeyPair("server.pem", "server.key")
	// if err != nil {
	// 	log.Fatalf("server: loadkeys: %s", err)
	// }

	ctx := Context{
		db: map[string]*worm.Value{},
	}

	server, err := worm.NewTCPServer("127.0.0.1:8081", nil, &ctx)
	if err != nil {
		log.Fatalln("Error attempting to start server:", err)
	}

	log.Println("Listening on:", server.Addr)

	if err := server.Run(); err != nil {
		server.Close()
		log.Fatal(err)
	}

	server.Close()
}
