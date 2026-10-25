// Package protocol defines the line-based wire format shared by clients and
// peers:
//
//	[FWD |REPL ]PING
//	[FWD |REPL ]GET <key>
//	[FWD |REPL ]DEL <key>
//	[FWD |REPL ]SET <key> <ttl_ms> <value: rest of line, may contain spaces>
package protocol

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

type Op string

const (
	OpGet  Op = "GET"
	OpSet  Op = "SET"
	OpDel  Op = "DEL"
	OpPing Op = "PING"
)

const (
	RespOK       = "OK"
	RespNotFound = "NOT_FOUND"
	RespDeleted  = "DELETED"
	RespPong     = "PONG"
)

const (
	prefixFwd  = "FWD"
	prefixRepl = "REPL"
)

type Command struct {
	Op                 Op
	Key, Value         string
	TTL                time.Duration
	Forwarded, Replica bool
}

// maxTTLMillis is the largest ttl_ms that still fits in a time.Duration.
const maxTTLMillis = math.MaxInt64 / int64(time.Millisecond)

// Parse turns one line (without its trailing "\n") into a Command. It never
// panics on malformed input: a bad line from one client must not crash the
// server.
func Parse(line string) (Command, error) {
	line = strings.TrimSuffix(line, "\r")
	var cmd Command

	op, rest, _ := strings.Cut(line, " ")
	switch strings.ToUpper(op) {
	case prefixFwd:
		cmd.Forwarded = true
		op, rest, _ = strings.Cut(rest, " ")
	case prefixRepl:
		cmd.Replica = true
		op, rest, _ = strings.Cut(rest, " ")
	}

	cmd.Op = Op(strings.ToUpper(op))
	switch cmd.Op {
	case OpPing:
		if rest != "" {
			return Command{}, errors.New("usage: PING")
		}
	case OpGet, OpDel:
		if rest == "" || strings.Contains(rest, " ") {
			return Command{}, errors.New("usage: " + string(cmd.Op) + " <key>")
		}
		cmd.Key = rest
	case OpSet:
		parts := strings.SplitN(rest, " ", 3)
		if len(parts) != 3 || parts[0] == "" {
			return Command{}, errors.New("usage: SET <key> <ttl_ms> <value>")
		}
		ms, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || ms < 0 || ms > maxTTLMillis {
			return Command{}, errors.New("invalid ttl_ms: " + parts[1])
		}
		cmd.Key, cmd.TTL, cmd.Value = parts[0], time.Duration(ms)*time.Millisecond, parts[2]
	default:
		return Command{}, errors.New("unknown command: " + op)
	}
	return cmd, nil
}

func Value(v string) string { return "VALUE " + v }
func Err(msg string) string { return "ERR " + msg }

// String renders c in wire format, so Parse(c.String()) == c.
func (c Command) String() string {
	var b strings.Builder
	switch {
	case c.Forwarded:
		b.WriteString(prefixFwd + " ")
	case c.Replica:
		b.WriteString(prefixRepl + " ")
	}
	b.WriteString(string(c.Op))
	switch c.Op {
	case OpGet, OpDel:
		b.WriteString(" " + c.Key)
	case OpSet:
		b.WriteString(" " + c.Key + " " + strconv.FormatInt(c.TTL.Milliseconds(), 10) + " " + c.Value)
	}
	return b.String()
}
