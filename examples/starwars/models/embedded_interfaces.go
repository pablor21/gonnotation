package models

// Add embedded interface test to the Star Wars example

// Base interfaces to embed
type Reader interface {
	Read() string
}

type Writer interface {
	Write(data string)
}

// Interface with embedded interfaces
type ReadWriter interface {
	Reader // embedded interface
	Writer // embedded interface
	Close() error
}

// Interface with mixed embedded and direct methods
type AdvancedReadWriter interface {
	ReadWriter // embedded interface
	Flush() error
	GetStatus() string
}
