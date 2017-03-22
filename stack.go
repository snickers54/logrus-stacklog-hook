package stklog

import (
	"fmt"
	"os"
	"runtime"
	"time"

	uuid "github.com/satori/go.uuid"
)

const (
	STACK_NOT_ENDED = "Can't attach current stack.\nDid you forget to use End() on your stack ?\nCalled from %s#%d"
	STACK_NOT_FOUND = "Can't find relevant stack for sending logs."
)

// Stack structure representing a logical block of logs
type Stack struct {
	ParentRequestID string                 `json:"parent_request_id"`
	Name            string                 `json:"name"`
	Extra           map[string]interface{} `json:"extra"`
	RequestID       string                 `json:"request_id"`
	Timestamp       string                 `json:"timestamp"`
	Line            int                    `json:"line"`
	File            string                 `json:"file"`
	Hostname        string                 `json:"hostname"`
	pushed          bool
}

// Let you set a custom requestID (unique identifier used to link stacks and logs)
// update our internal mapping of GoroutineID -> requestID
func (self *Stack) SetRequestID(requestID string) *Stack {
	self.RequestID = requestID
	mapping[getGID()] = requestID
	return self
}

// Set extra fields on your stack, for instance HTTP HEADERS for an API or webservice
func (self *Stack) SetFields(fields map[string]interface{}) *Stack {
	self.Extra = fields
	return self
}

// Set a name on your stack, to identify it quickly in Stklog.io UI
func (self *Stack) SetName(name string) *Stack {
	self.Name = name
	return self
}

// End is sending this stack through a channel listened by writerLoop
func (self *Stack) End() {
	self.pushed = true
	chanBuffer <- iMessage(self)
}

// Attach is used when you want to log inside a goroutine as part of the same context
// Attach create a new stack attaching the current stack as a parent of this new stack
// You can attach only a stack that was ended (see End())
// In case you want to log inside a goroutine with being part of a parent Stack, just call CreateStack at the begining of this goroutine
func (self *Stack) Attach() (*Stack, error) {
	if self.pushed == false {
		// since the attach is not working, we ensure that any call to logrus inside this goroutine
		// will be discarded, even if the internal goroutine id system reuse old ids and therefore is already in our mapping table..
		delete(mapping, getGID())
		_, file, line, _ := runtime.Caller(1)
		return nil, fmt.Errorf(STACK_NOT_ENDED, file, line)
	}
	stack := CreateStack()
	stack.ParentRequestID = self.RequestID
	return stack, nil
}

// Create a new stack, will map a generated or custom requestID (see SetRequestID) with a Goroutine ID
// If you call CreateStack in the same goroutine, without using the same RequestID, it will overwrite this RequestID with a generated one
func CreateStack() *Stack {
	// we generate a requestID as a uuid4
	requestID := uuid.NewV4().String()
	mapping[getGID()] = requestID
	host, err := os.Hostname()
	if err != nil {
		host = "localhost"
	}
	_, file, line, _ := runtime.Caller(1)
	return &Stack{
		pushed:    false,
		RequestID: requestID,
		Timestamp: time.Now().Format(time.RFC3339),
		File:      file,
		Line:      line,
		Hostname:  host,
		Extra:     map[string]interface{}{},
	}
}
