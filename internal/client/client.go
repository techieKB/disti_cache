// Package client sends protocol lines to a DistiCache node.
package client

import (
	"bufio"
	"io"
	"net"
	"strings"
	"time"
)

// Do sends one command line to addr and returns the single-line response
// without its trailing newline. It dials a fresh TCP connection per call;
func Do(addr, line string, timeout time.Duration) (string, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return "", err
	}
	if _, err := io.WriteString(conn, line+"\n"); err != nil {
		return "", err
	}
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(resp, "\n"), nil
}
