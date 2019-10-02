# worm

A Redis-protocol server framework for Go

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
