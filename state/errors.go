package state

import "fmt"

type SyntaxError struct {
	Chunk  string
	Line   int
	Column int
	Err    error
}

func (e *SyntaxError) Error() string { return e.Err.Error() }
func (e *SyntaxError) Unwrap() error { return e.Err }

type RuntimeError struct {
	Chunk  string
	Line   int
	Column int
	Stack  []string
	Err    error
}

func (e *RuntimeError) Error() string { return e.Err.Error() }
func (e *RuntimeError) Unwrap() error { return e.Err }

type BridgeError struct {
	Name   string
	Line   int
	Column int
	Err    error
}

func (e *BridgeError) Error() string { return e.Err.Error() }
func (e *BridgeError) Unwrap() error { return e.Err }

type ContextError struct {
	Err error
}

func (e *ContextError) Error() string { return e.Err.Error() }
func (e *ContextError) Unwrap() error { return e.Err }

type ExitError struct {
	Code int
}

func (e *ExitError) Error() string { return fmt.Sprintf("os.exit(%d)", e.Code) }
