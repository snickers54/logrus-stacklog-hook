package stklog

import (
	"fmt"
	"net/http"
	"time"

	"github.com/parnurzeal/gorequest"
)

// Global package channel accepting iMessage interface
// We accept either a log (LogMessage) or a "stack" (Stack)
var chanBuffer = make(chan iMessage)
var running = false

const (
	STKLOG_HOST            = "https://stklog.io"
	STKLOG_STACKS_ENDPOINT = "stacks"
	STKLOG_LOGS_ENDPOINT   = "logs"
)

// empty interface, but I prefer defining it
type iMessage interface{}

// Normalized log adapted for Stklog API
type LogMessage struct {
	Level     int32                  `json:"level"`
	Extra     map[string]interface{} `json:"extra"`
	Message   string                 `json:"message"`
	RequestID string                 `json:"request_id"`
	Timestamp string                 `json:"timestamp"`
	Line      int                    `json:"line"`
	File      string                 `json:"file"`
}

// factory to create stdRequests with common options to both stacks and logs
func newStdRequest() *gorequest.SuperAgent {
	return gorequest.New().
		Timeout(3*time.Second).
		Retry(2, 1*time.Second, http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout)
}

// Init pre constructed requests and launch a writerLoop goroutine to bufferise and send logs
func start(stklogProjectKey string) {
	if running == true {
		fmt.Printf("You already have a running hook.\n")
		return
	}
	stacksRequest := newStdRequest().Post(fmt.Sprintf("%s/%s", STKLOG_HOST, STKLOG_STACKS_ENDPOINT)).Set("Stklog-Project-Key", stklogProjectKey).
		Set("Content-Type", "application/json")
	logsRequest := newStdRequest().Post(fmt.Sprintf("%s/%s", STKLOG_HOST, STKLOG_LOGS_ENDPOINT)).Set("Stklog-Project-Key", stklogProjectKey).
		Set("Content-Type", "application/json")
	go writerLoop(logsRequest, stacksRequest)
	running = true
}

// Bufferise logs and stacks
// Send requests every 5seconds and empty the buffer
func writerLoop(logsRequest, stacksRequest *gorequest.SuperAgent) {
	buffer := struct {
		Stacks []Stack
		Logs   []LogMessage
	}{}
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case toSend := <-chanBuffer:
			switch value := toSend.(type) {
			case *LogMessage:
				buffer.Logs = append(buffer.Logs, *toSend.(*LogMessage))
			case *Stack:
				buffer.Stacks = append(buffer.Stacks, *toSend.(*Stack))
			default:
				fmt.Printf("%+v is an invalid iMessage object.\n", value)
			}
		case <-ticker.C:
			if len(buffer.Stacks) > 0 {
				execRequest(stacksRequest, buffer.Stacks)
				buffer.Stacks = []Stack{}
			}
			if len(buffer.Logs) > 0 {
				execRequest(logsRequest, buffer.Logs)
				buffer.Logs = []LogMessage{}
			}
		}
	}
}

// wrapper to execute the requests and deal with common errors
func execRequest(request *gorequest.SuperAgent, array interface{}) {
	resp, _, errs := request.Send(array).End()
	if resp.StatusCode == http.StatusUnauthorized {
		fmt.Println("Stklog project key is invalid.")
		return
	}
	if resp.StatusCode != http.StatusCreated {
		fmt.Printf("Couldn't send request to %s\n, errors : %s\n", STKLOG_HOST, errs)
	}
}
