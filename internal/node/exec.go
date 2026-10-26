// Package node turns parsed commands into store operations.
package node

import (
	"github.com/techieKB/disti_cache/internal/protocol"
	"github.com/techieKB/disti_cache/internal/store"
)

// Execute runs cmd against the local store only. No routing, no replication.
func Execute(st *store.Store, cmd protocol.Command) string {
	switch cmd.Op {
	case protocol.OpPing:
		return protocol.RespPong
	case protocol.OpGet:
		if v, ok := st.Get(cmd.Key); ok {
			return protocol.Value(v)
		}
		return protocol.RespNotFound
	case protocol.OpSet:
		st.Set(cmd.Key, cmd.Value, cmd.TTL)
		return protocol.RespOK
	case protocol.OpDel:
		if st.Delete(cmd.Key) {
			return protocol.RespDeleted
		}
		return protocol.RespNotFound
	}
	return protocol.Err("unsupported op " + string(cmd.Op))
}
