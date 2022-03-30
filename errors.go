package enet

import "fmt"

var (
	enableDebug = true

	ErrUnsupportedFlags  = Error("unsupport flag")
	ErrNotImplemented    = Error("not implemented")
	ErrInvalidStatus     = Error("invalid status")
	ErrInvalidPacketSize = Error("invalid packet size: %v")
	ErrAssert            = Error("assert false")
)

type Error string

func assert(v bool) {
	if !v {
		panic(ErrAssert.f())
	}
}

func assure(v bool, format string, a ...interface{}) {
	if !v {
		fmt.Printf(format, a...)
	}
}

func PanicError(format string, a ...interface{}) {
	panic(fmt.Errorf(format, a...))
}

func (err Error) f(a ...interface{}) error {
	return fmt.Errorf(string(err), a...)
}

func debugf(format string, a ...interface{}) {
	if enableDebug {
		fmt.Printf(format, a...)
	}
}
