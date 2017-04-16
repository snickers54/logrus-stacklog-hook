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
	BATCH_SIZE             = 50
	MAX_REQUESTS           = 10
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

// Init pre constructed requests and launch a writerLoop goroutine to bufferise and send logs
func start(stklogProjectKey string) {
	if running == true {
		fmt.Printf("[STKLOG] You already have a running hook.\n")
		return
	}
	go writerLoop(stklogProjectKey)
	running = true
}

// Bufferise logs and stacks
// Send requests every 5seconds and empty the buffer
func writerLoop(projectKey string) {
	ticker := time.NewTicker(1 * time.Second)
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
				fmt.Printf("[STKLOG] %+v is an invalid iMessage object.\n", value)
			}
		case <-ticker.C:
			go send(projectKey)
		case <-flusher:
			count := (max(len(buffer.Stacks), len(buffer.Logs)) / BATCH_SIZE) + 1
			fmt.Printf("[STKLOG] Flushing stacks/logs from buffer. %d stacks and %d logs.\n", len(buffer.Stacks), len(buffer.Logs))
			for i := 0; i < count; i++ {
				fmt.Printf("[STKLOG] %d%%\n", 100*i/count)
				send(projectKey)
			}
			// We don't close the channels, since if it writes into it before the program actually die/quit, it will panic ..
			break infiniteLoop
		}
	}
	flusher <- true
}

func cloneResetBuffers() ([]Stack, []LogMessage) {
	buffer.mutex.Lock()
	maxStacks := min(BATCH_SIZE*MAX_REQUESTS, len(buffer.Stacks))
	maxLogs := min(BATCH_SIZE*MAX_REQUESTS, len(buffer.Logs))
	stacks := make([]Stack, maxStacks)
	logs := make([]LogMessage, maxLogs)
	copy(stacks, buffer.Stacks[:maxStacks])
	copy(logs, buffer.Logs[:maxLogs])
	if maxStacks == len(buffer.Stacks) {
		buffer.Stacks = nil
	} else {
		buffer.Stacks = buffer.Stacks[maxStacks:]
	}
	if maxLogs == len(buffer.Logs) {
		buffer.Logs = nil
	} else {
		buffer.Logs = buffer.Logs[maxLogs:]
	}
	buffer.mutex.Unlock()
	return stacks, logs
}

// execute requests to send stacks and logs to the API and reset the buffers after
func send(stklogProjectKey string) {
	// it's quicker to perform a copy of our slices and then set the original to nil and unlock our mutex for next appends
	// unfortunately in case of flush it's unneeded operations but whatever
	stacks, logs := cloneResetBuffers()
	if length := len(stacks); length > 0 {
		stacksRequest := gorequest.New().Post(fmt.Sprintf("%s/%s", STKLOG_HOST, STKLOG_STACKS_ENDPOINT)).Set("Stklog-Project-Key", stklogProjectKey).
			Set("Content-Type", "application/json")
		stacksRequest.Transport.DisableKeepAlives = true
		for i := 0; i < length; i += min(BATCH_SIZE, length-i) {
			execRequest(stacksRequest, stacks[i:min(BATCH_SIZE+i, length)])
		}
	}
	if length := len(logs); length > 0 {
		logsRequest := gorequest.New().Post(fmt.Sprintf("%s/%s", STKLOG_HOST, STKLOG_LOGS_ENDPOINT)).Set("Stklog-Project-Key", stklogProjectKey).
			Set("Content-Type", "application/json")
		logsRequest.Transport.DisableKeepAlives = true
		for i := 0; i < length; i += min(BATCH_SIZE, length-i) {
			execRequest(logsRequest, logs[i:min(BATCH_SIZE+i, length)])
		}
	}
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

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}
