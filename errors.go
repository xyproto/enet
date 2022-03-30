package enet

import "fmt"

var (
	EneterrUnsupportedFlags  = Eneterror("unsupport flag")
	EneterrNotImplemented    = Eneterror("not implemented")
	EneterrInvalidStatus     = Eneterror("invalid status")
	EneterrInvalidPacketSize = Eneterror("invalid packet size: %v")
	EneterrAssert            = Eneterror("assert false")

	enableDebug = true
)

type Eneterror string

func assert(v bool) {
	if !v {
		panic(EneterrAssert.f())
	}
}
func assure(v bool, format string, a ...interface{}) {
	if !v {
		fmt.Printf(format, a...)
	}
}
func EnetpanicError(format string, a ...interface{}) {
	panic(fmt.Errorf(format, a...))
}

func (err Eneterror) f(a ...interface{}) error {
	return fmt.Errorf(string(err), a...)
}

func debugf(format string, a ...interface{}) {
	if enableDebug {
		fmt.Printf(format, a...)
	}
}
