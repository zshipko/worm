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

type Message struct {
	Kind  MessageKind
	Type  string
	Value *Value
	User  *User
}
