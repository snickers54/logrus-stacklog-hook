package stklog

import (
	"bytes"
	"errors"
	"runtime"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

// hook struct implementing Logrus Hook interface (https://github.com/sirupsen/logrus/blob/master/hooks.go)
// containing logLevels defined by SetLevel
type StklogHook struct {
	logLevels []logrus.Level
}

// Factory to create a new Hook
// will initiate a goroutine to bufferise and send logs to stklog.io
func NewStklogHook(apiKey string) *StklogHook {
	start(apiKey)
	return &StklogHook{}
}

// Standard method called by logrus when a log is written
// we normalize the log and send it to a channel to be bufferised and sent later
func (hook *StklogHook) Fire(entry *logrus.Entry) error {
	message := bytes.TrimSpace([]byte(entry.Message))
	requestID, ok := mapping[getGID()]
	if ok == false {
		return errors.New(STACK_NOT_FOUND)
	}
	file, line := getCaller()
	logMessage := &LogMessage{
		// logrus levels are lower than syslog by 2
		Level:     int32(entry.Level) + 2,
		Extra:     entry.Data,
		Message:   string(message),
		Timestamp: entry.Time.Format(time.RFC3339),
		File:      file,
		Line:      line,
		RequestID: requestID,
	}
	chanBuffer <- iMessage(logMessage)
	return nil
}

// function to retrieve file and line of the logrus call from the Stack
func getCaller() (file string, line int) {
	callDepth := 1
	var ok bool
	for {
		_, file, line, ok = runtime.Caller(callDepth)
		if !ok {
			break
		}
		if strings.HasSuffix(file, "logrus/logger.go") {
			callDepth++
			_, file, line, ok = runtime.Caller(callDepth)
			break
		}
		callDepth++
	}
	return
}

// Standard method to implement for the hook interface, telling logrus which levels it needs to call us for
func (hook *StklogHook) Levels() []logrus.Level {
	if len(hook.logLevels) == 0 {
		return logrus.AllLevels
	}
	return hook.logLevels
}

// Custom method for user to define from which level he/she wants to logs to Stklog
func (hook *StklogHook) SetLevel(level logrus.Level) {
	for _, element := range logrus.AllLevels {
		if int32(element) <= int32(level) {
			hook.logLevels = append(hook.logLevels, element)
		}
	}
}
