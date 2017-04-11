package stklog

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/parnurzeal/gorequest"
)

// Global package channel accepting iMessage interface
// We accept either a log (LogMessage) or a "stack" (Stack)
type msgBuffer struct {
	Stacks []Stack
	Logs   []LogMessage
	mutex  sync.Mutex
}

var mutexBuffer = &sync.Mutex{}
var chanBuffer = make(chan iMessage)
var flusher = make(chan bool)
var running = false
var buffer = msgBuffer{}

const (
	STKLOG_HOST            = "https://api.stklog.io"
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
	go writerLoop(stklogProjectKey)
	running = true
}

// Bufferise logs and stacks
// Send requests every 5seconds and empty the buffer
func writerLoop(projectKey string) {
	ticker := time.NewTicker(5 * time.Second)
infiniteLoop:
	for {
		select {
		case toSend := <-chanBuffer:
			switch value := toSend.(type) {
			case *LogMessage:
				buffer.mutex.Lock()
				buffer.Logs = append(buffer.Logs, *toSend.(*LogMessage))
				buffer.mutex.Unlock()
			case *Stack:
				buffer.mutex.Lock()
				buffer.Stacks = append(buffer.Stacks, *toSend.(*Stack))
				buffer.mutex.Unlock()
			default:
				fmt.Printf("%+v is an invalid iMessage object.\n", value)
			}
		case <-ticker.C:
			go send(projectKey)
		case <-flusher:
			send(projectKey)
			// We don't close the channels, since if it writes into it before the program actually die/quit, it will panic ..
			break infiniteLoop
		}
	}
	flusher <- true
}

// execute requests to send stacks and logs to the API and reset the buffers after
func send(stklogProjectKey string) {
	// it's quicker to perform a copy of our slices and then set the original to nil and unlock our mutex for next appends
	// unfortunately in case of flush it's unneeded operations but whatever
	var stacks []Stack = []Stack{}
	var logs []LogMessage = []LogMessage{}
	buffer.mutex.Lock()
	copy(stacks, buffer.Stacks)
	buffer.Stacks = nil
	copy(logs, buffer.Logs)
	buffer.Logs = nil
	buffer.mutex.Unlock()

	if length := len(stacks); length > 0 {
		stacksRequest := newStdRequest().Post(fmt.Sprintf("%s/%s", STKLOG_HOST, STKLOG_STACKS_ENDPOINT)).Set("Stklog-Project-Key", stklogProjectKey).
			Set("Content-Type", "application/json")
		for i := 0; i < length; i += min(250, length-i) {
			execRequest(stacksRequest, stacks[i:min(250+i, length)])
		}
	}
	if length := len(logs); length > 0 {
		logsRequest := newStdRequest().Post(fmt.Sprintf("%s/%s", STKLOG_HOST, STKLOG_LOGS_ENDPOINT)).Set("Stklog-Project-Key", stklogProjectKey).
			Set("Content-Type", "application/json")
		for i := 0; i < length; i += min(250, length-i) {
			execRequest(logsRequest, logs[i:min(250+i, length)])
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
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Couldn't send request to %s\n, errors : %s\n", STKLOG_HOST, errs)
	}
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
