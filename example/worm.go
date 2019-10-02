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

func (cmd *Context) Get(client *worm.Client, key *worm.Value) error {
	lock.Lock()
	defer lock.Unlock()
	return client.WriteValue(cmd.db[key.ToString()])
}

type cmdSet struct {
	db map[string]*worm.Value
}

func (cmd *Context) Set(client *worm.Client, key *worm.Value, value *worm.Value) error {
	lock.Lock()
	defer lock.Unlock()
	cmd.db[key.ToString()] = value
	return client.WriteOK()
}

func (cmd *Context) Del(client *worm.Client, keys ...*worm.Value) error {
	lock.Lock()
	defer lock.Unlock()

	if len(keys) == 0 {
		return worm.ErrNotEnoughArguments
	}

	count := 0

	for _, k := range keys {
		exists := cmd.db[k.ToString()] != nil
		delete(cmd.db, k.ToString())
		if exists {
			count += 1
		}
	}

	if count == 1 && len(keys) == 2 {
		return client.WriteOK()
	}

	return client.WriteValue(worm.New(int64(count)))
}

func main() {

	// NOTE: uncomment, and pass `cert` to NewTCPServer to create
	// and use a self-signed SSL certificate
	//cert, err := worm.GenerateSelfSignedSSLCert("./worm")
	//if err != nil {
	//	log.Fatal("Unable to generate/load self-signed certs: ", err)
	//}

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
