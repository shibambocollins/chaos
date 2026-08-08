package raft

import "math/rand"

// handleMessage is deliberately unimplemented here — see Module 4
// (phase2/04-handle-message). Section 7 of the context doc is the contract;
// this stub exists only so Step() and its tests compile in isolation.
func (n *NodeState) handleMessage(from int, msg *RaftMessage, rng *rand.Rand) []Outbound {
	return nil
}
