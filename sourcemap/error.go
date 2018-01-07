package sourcemap

import (
	"fmt"
)

type Enum string

const (
	InitError   Enum = "InitError"
	AccessError Enum = "AccessError"
	ParseError  Enum = "ParseError"
	MapError    Enum = "MapError"
	KeyError    Enum = "KeyError"
)

type Error struct {
	Msg  string
	Kind Enum
}

func (e Error) Error() string {
	return fmt.Sprintf("%s", e.Msg)
}
