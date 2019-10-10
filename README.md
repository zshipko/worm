# worm

A reflection-based RESP3 server framework for Go

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

func (c *MyCommands) Example(client *worm.Client, args ...*worm.Value) error {
  return client.WriteValue(NewArray(args))
}

func (c *MyCommands) Example2(client *worm.Client, arg1 *worm.Value, arg2 *worm.Value) error {
  if err := client.WriteArrayHeader(2); err != nil {
    return err
  }

  if err := client.WriteValue(arg1); err != nil {
    return err
  }

  return client.WriteValue(arg2)
}

func (c *MyCommands) SomethingElse(i int) int {
  return i + 1
}
```

In the example above, `MyCommands` exports two `worm` commands named `Example` and `Example2`. `SomethingElse`
isn't converted to a command because it has incompatible arguments.

A command must start with a `*worm.Client` argument and contain any number of `*worm.Value` arguments, including
variadic arguments.

Once you have written all your commands, you can easily create a new server:

```go
ctx := MyCommands {}
server, err := worm.NewTCPServer("127.0.0.1:8081", nil, &ctx)
```

And run it:

```go
server.Run()
```
