// Package freenect implements a Go binding for the libfreenect library.
package freenect

/*
#cgo CFLAGS: -I/opt/homebrew/include
#cgo LDFLAGS: -L/opt/homebrew/lib -lfreenect
#include <libfreenect/libfreenect.h>
*/
import "C"

func init() {

	contexts = make(map[*C.freenect_context]*Context)
	devices = make(map[*C.freenect_device]*Device)
}
