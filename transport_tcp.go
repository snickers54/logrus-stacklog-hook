package stklog

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/vmihailenco/msgpack"
)

const (
	STKLOG_DOMAIN   = "api.stklog.io"
	STKLOG_TCP_PORT = 4242
)

type transportTCP struct {
	transport
	TCPConn net.Conn
}

func (self *transportTCP) Init() {
	self.projectKey = self.GetProjectKey()
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", STKLOG_DOMAIN, STKLOG_TCP_PORT), &tls.Config{})
	if err != nil {
		fmt.Printf("[STKLOG] Couldn't connect to %s:%d\n with error : %s\n", STKLOG_DOMAIN, STKLOG_TCP_PORT, err)
		return
	}
	self.TCPConn = conn
	go func() {
		reader := bufio.NewReader(self.TCPConn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			fmt.Printf("[STKLOG] %s\n", line)
		}
	}()
}

func (self *transportTCP) Flush() {
	if len(buffer.Logs) == 0 && len(buffer.Stacks) == 0 {
		return
	}
	fmt.Printf("[STKLOG] Flushing stacks/logs from buffer. %d stacks and %d logs.\n", len(buffer.Stacks), len(buffer.Logs))
	self.Send()
}
func (self *transportTCP) write(msgArray [][]byte, retry bool) {
	msg := bytes.Join(msgArray, []byte("\t"))
	msg = append(msg, '\n')
	if n, err := self.TCPConn.Write(msg); n != len(msg) || err != nil {
		fmt.Printf("[STKLOG] Couldn't write properly on the TCP socket. size sent : %d, size of input : %d, error : %s\n", n, len(msg), err)
		self.TCPConn.Close()
		self.Init()
		if retry == true {
			self.write(msgArray, false)
		}
	}
}

func (self *transportTCP) Send() {
	stacks, logs := cloneResetBuffers()
	for _, stack := range stacks {
		content, err := msgpack.Marshal(stack)
		if err != nil {
			fmt.Printf("[STKLOG] %s\n", err)
			continue
		}
		msg := [][]byte{[]byte(self.GetProjectKey()), []byte("stack"), content}
		self.write(msg, true)
	}
	for _, logMsg := range logs {
		content, err := msgpack.Marshal(logMsg)
		if err != nil {
			fmt.Printf("[STKLOG] %s\n", err)
			continue
		}
		msg := [][]byte{[]byte(self.GetProjectKey()), []byte("log"), content}
		self.write(msg, true)
	}
}
