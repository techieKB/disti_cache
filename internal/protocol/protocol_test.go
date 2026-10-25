package protocol

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		line string
		want Command
	}{
		{"PING", Command{Op: OpPing}},
		{"ping", Command{Op: OpPing}},
		{"GET user:1", Command{Op: OpGet, Key: "user:1"}},
		{"GET k\r", Command{Op: OpGet, Key: "k"}},
		{"DEL k", Command{Op: OpDel, Key: "k"}},
		{"SET k 0 hello world", Command{Op: OpSet, Key: "k", Value: "hello world"}},
		{"SET k 1500 v", Command{Op: OpSet, Key: "k", TTL: 1500 * time.Millisecond, Value: "v"}},
		{"SET k 0 ", Command{Op: OpSet, Key: "k"}},
		{"FWD GET k", Command{Op: OpGet, Key: "k", Forwarded: true}},
		{"REPL SET k 0 v", Command{Op: OpSet, Key: "k", Value: "v", Replica: true}},
	}
	for _, tt := range tests {
		got, err := Parse(tt.line)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.line, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %+v, want %+v", tt.line, got, tt.want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, line := range []string{
		"",
		"   ",
		"FLY k",
		"PING extra",
		"GET",
		"GET ",
		"GET a b",
		"SET k",
		"SET k 0",
		"SET  0 v",
		"SET k abc v",
		"SET k -5 v",
		"SET k 99999999999999999 v", // would overflow time.Duration
		"FWD REPL GET k",
	} {
		if cmd, err := Parse(line); err == nil {
			t.Errorf("Parse(%q) = %+v, want error", line, cmd)
		}
	}
}

func TestStringRoundTrips(t *testing.T) {
	for _, c := range []Command{
		{Op: OpPing},
		{Op: OpGet, Key: "k"},
		{Op: OpDel, Key: "k", Replica: true},
		{Op: OpSet, Key: "k", TTL: 2 * time.Second, Value: "a b c", Forwarded: true},
		{Op: OpSet, Key: "k"},
	} {
		got, err := Parse(c.String())
		if err != nil || got != c {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", c.String(), got, err, c)
		}
	}
}
