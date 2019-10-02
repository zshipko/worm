# worm

A RESP3 server framework for Go

## Protocol

`worm` implements the majority of the RESP3 protocol, however the following components are not yet implemented:
- Streaming types
- Attribute type
- Non-string map keys

## Getting started

`worm` uses reflection to build a map of commands based on the methods of a struct value:

```go
type MyCommands struct {
  db: map[string]*Value,
  lock: sync.Mutex,
}

func (c *MyCommands) Example(client *worm.Client, args []*worm.Value) error {
  return client.WriteOK()
}

func (c *MyCommands) SomethingElse(i int) int {
  return i + 1
}
```

In the example above, `MyCommands` exports a single `worm` command named `Example`. `SomethingElse`
isn't converted to a command because it has incompatible arguments and return type.

Once you have written all your commands, you can easily create a new server:

```go
ctx := MyCommands {}
server, err := worm.NewTCPServer("127.0.0.1:8081", nil, &ctx)
```

And run it:

```go
server.Run()
```
