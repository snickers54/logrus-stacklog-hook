package stklog

import (
	"bytes"
	"runtime"
)

// in memory table mapping between goroutine id (stack ID) and generated request ID for stklog
var mapping = map[string]string{}

// Source : http://blog.sgmansfield.com/2015/12/goroutine-ids/
// Thanks to Scott Mansfield
// get current Goroutine ID as a string
func getGID() string {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	return string(b)
}

// return current operating requestID (goroutine ID dependent)
func GetCurrentRequestID() (string, bool) {
	value, ok := mapping[getGID()]
	return value, ok
}
