package main

import (
	"log"

	"github.com/zshipko/worm"
)

import (
	"os"
	"os/signal"
	"syscall"
)

type cmdGet struct{}

func (_ *cmdGet) Exec(db *worm.DB, client *worm.Client, args []*worm.Value) error {
	if len(args) < 2 {
		return worm.ErrNotEnoughArguments
	}

	k := args[1].ToString()

	v, err := db.Get(k)
	if err != nil {
		return err
	}

	return client.WriteValue(v)
}

type cmdSet struct{}

func (_ *cmdSet) Exec(db *worm.DB, client *worm.Client, args []*worm.Value) error {
	if len(args) < 3 {
		return worm.ErrNotEnoughArguments
	}

	k := args[1].ToString()
	v := args[2]

	if err := db.Put(k, v); err != nil {
		return err
	}

	return client.WriteOK()
}

type cmdDel struct{}

func (_ *cmdDel) Exec(db *worm.DB, client *worm.Client, args []*worm.Value) error {
	if len(args) < 2 {
		return worm.ErrNotEnoughArguments
	}

	count := 0

	for _, k := range args[1:] {
		ks := k.ToString()
		if ok, _ := db.Store.Has([]byte(ks)); ok {
			if err := db.Delete(ks); err == nil {
				count++
			}
		}
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

	server, err := worm.NewTCPServer("127.0.0.1:8081", nil)
	if err != nil {
		log.Fatalln("Error attempting to start server:", err)
	}

	log.Println("Listening on:", server.Addr)
	server.Commands["get"] = &cmdGet{}
	server.Commands["set"] = &cmdSet{}
	server.Commands["del"] = &cmdDel{}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			sig := <-sigs
			if sig == os.Interrupt || sig == os.Kill {
				server.DB.Close()
				os.Exit(0)
			}
		}
	}()

	if err := server.Run(); err != nil {
		server.DB.Close()
		log.Fatal(err)
	}

	server.DB.Close()
}
