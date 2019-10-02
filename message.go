package worm

type MessageKind int

// TODO: streaming types + Hello

const (
	Default MessageKind = iota
	Verbatim
	SetReply
	Push
	Hello
)

type User struct {
	Name     string
	Password string
}

type Message struct {
	Kind  MessageKind
	Type  string
	Value *Value
	User  *User
}
