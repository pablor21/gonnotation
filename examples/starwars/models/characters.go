package models

import (
	"context"
	"time"

	"github.gom/pablor21/gonnotation/examples/starwars/base"
)

// Character represents a character in the Star Wars universe
// This is a custom type that can be used to represent characters in the Star Wars universe
// @schema("Character")
type Character interface {
	GetName() (ret string, err error)
}

type Node[T Character, P any] struct {
	Value T
	Key   P
	Next  *Node[T, P]
	Prev  *Node[T, P]
}

type XNode Node[Character, int]

type SimpleAlias = string

// type Droid struct {
// 	// Name of the droid character
// 	// @field("name")
// 	Name        string `json:"name" schema:"name"`
// 	PrimaryFunc string
// 	CreatedAt   *time.Time
// // }

// Human represents a human character
// @schema("Human")
type Human struct {
	base.BaseModel
	// Name of the human character
	// @field("name")
	Name string `json:"name" schema:"name"`

	CreatedAt *time.Time
	Friends   []Human `json:"friends" schema:"friends"`
}

func (h Human) GetName(ctx context.Context) string {
	return h.Name
}

// Test is a test function
// @function("Test")
func Test(name string) string {
	return name
}

// // Droid represents a droid character
// // @schema("Droid")
// type Droid struct {
// 	Name        string
// 	PrimaryFunc string
// }

// func (d Droid) GetName() string {
// 	return d.Name
// }
