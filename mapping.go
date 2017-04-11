package stklog

import (
	"bytes"
	"runtime"
	"sync"
)

// in memory table mapping between goroutine id (stack ID) and generated request ID for stklog
var mapping = map[string]string{}
var mutexMapping = &sync.Mutex{}

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
	mutexMapping.Lock()
	value, ok := mapping[getGID()]
	mutexMapping.Unlock()
	return value, ok
}
