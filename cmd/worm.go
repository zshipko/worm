package main

import (
	"log"

	"github.com/zshipko/worm"
)

func main() {
	// cert, err := LoadX509KeyPair("server.pem", "server.key")
	// if err != nil {
	// 	log.Fatalf("server: loadkeys: %s", err)
	// }

	server, err := worm.NewTCPServer("127.0.0.1:8081", nil)
	if err != nil {
		log.Fatalln("Error attempting to start server on:", server.Addr, err)
	}
	log.Println("Listening on:", server.Addr)
	server.Commands["get"] = func(client *worm.Client, args []*worm.Value) error {
		return client.WriteOK()
	}
	server.Run()
}
