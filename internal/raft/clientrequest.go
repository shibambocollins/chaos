package raft

// handleClientRequest implements EventClientRequest: a client submitted cmd
// to this node. Only a Leader can accept it — Chaos doesn't model a
// redirect-to-leader response, so a non-Leader silently drops the request,
// same as it would if the client's RPC to the wrong node simply timed out.
//
// On success, the entry is appended to the leader's own log (index = tip+1,
// its own current term), persisted, and pushed out to every peer
// immediately via replicateTo rather than waiting for the next heartbeat
// tick — matching the same "become leader, replicate now" pattern
// becomeLeader already uses.
func (n *NodeState) handleClientRequest(cmd []byte) []Outbound {
	if n.Role != Leader {
		return nil
	}

	lastIndex, _ := n.lastLogIndexAndTerm()
	n.Log = append(n.Log, LogEntry{
		Term:    n.CurrentTerm,
		Index:   lastIndex + 1,
		Command: cmd,
	})

	out := []Outbound{n.persistOutbound()}
	for _, peer := range n.Peers {
		out = append(out, n.replicateTo(peer))
	}
	return out
}
