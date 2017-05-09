package stklog

import (
	"fmt"
	"net/http"

	"github.com/parnurzeal/gorequest"
)

const (
	KEY_BATCH_SIZE         = "http_batch_size"
	MAX_BATCH_SIZE         = 500
	DEFAULT_BATCH_SIZE     = 200
	STKLOG_HOST            = "https://api.stklog.io"
	STKLOG_STACKS_ENDPOINT = "stacks"
	STKLOG_LOGS_ENDPOINT   = "logs"
)

type transportHTTP struct {
	transport
	batchSize int
}

func (self *transportHTTP) Init() {
	self.batchSize = self.GetBatchSize()
	self.projectKey = self.GetProjectKey()
}

func (self *transportHTTP) GetBatchSize() int {
	if valueInterface, ok := self.GetOption(KEY_BATCH_SIZE); ok {
		switch valueInterface.(type) {
		case int:
			if valueInterface.(int) <= MAX_BATCH_SIZE && valueInterface.(int) > 0 {
				return valueInterface.(int)
			}
			break
		default:
			break
		}
	}
	return DEFAULT_BATCH_SIZE
}

func (self *transportHTTP) Flush() {
	count := (max(len(buffer.Stacks), len(buffer.Logs)) / self.batchSize) + 1
	fmt.Printf("[STKLOG] Flushing stacks/logs from buffer. %d stacks and %d logs.\n", len(buffer.Stacks), len(buffer.Logs))
	for i := 0; i < count; i++ {
		self.Send()
		fmt.Printf("[STKLOG] %d%%\n", 100*i/count)
	}
}

// execute requests to send stacks and logs to the API and reset the buffers after
func (self *transportHTTP) Send() {
	// it's quicker to perform a copy of our slices and then set the original to nil and unlock our mutex for next appends
	// unfortunately in case of flush it's unneeded operations but whatever
	stacks, logs := cloneResetBuffers()
	if length := len(stacks); length > 0 {
		stacksRequest := self.prepare(STKLOG_STACKS_ENDPOINT)
		for i := 0; i < length; i += min(self.batchSize, length-i) {
			execRequest(stacksRequest, stacks[i:min(self.batchSize+i, length)])
		}
	}
	if length := len(logs); length > 0 {
		logsRequest := self.prepare(STKLOG_LOGS_ENDPOINT)
		for i := 0; i < length; i += min(self.batchSize, length-i) {
			execRequest(logsRequest, logs[i:min(self.batchSize+i, length)])
		}
	}
}

func (self *transportHTTP) prepare(endpoint string) *gorequest.SuperAgent {
	objectRequest := gorequest.New().Post(fmt.Sprintf("%s/%s", STKLOG_HOST, endpoint)).Set("X-Stklog-Project-Key", self.GetProjectKey()).
		Set("Content-Type", "application/json")
	objectRequest.Transport.DisableKeepAlives = true
	return objectRequest
}

// wrapper to execute the requests and deal with common errors
func execRequest(request *gorequest.SuperAgent, array interface{}) {
	resp, _, errs := request.Send(array).End()
	if resp == nil {
		fmt.Println("[STKLOG] An unexpected error happened.", errs)
		return
	}
	if resp.StatusCode == http.StatusUnauthorized {
		fmt.Println("[STKLOG] project key is invalid.")
		return
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[STKLOG] Couldn't send request to %s\n, errors : %s\n", STKLOG_HOST, errs)
	}
}
