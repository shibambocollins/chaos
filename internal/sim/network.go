package sim

import (
	"math/rand"

	"chaos/internal/raft"
)

const (
	// latencyMin/Max bound the randomized simulated network delay applied
	// to every message. Randomization here is what makes reorder fall out
	// for free (context doc §4): two messages sent in order can still
	// arrive out of order if the later one samples a shorter latency, with
	// no separate "reorder mode" anywhere in the node or simulator.
	latencyMin raft.Time = 1
	latencyMax raft.Time = 4
)

func sampleLatency(rng *rand.Rand) raft.Time {
	return latencyMin + raft.Time(rng.Int63n(int64(latencyMax-latencyMin+1)))
}

// scheduleDelivery is the single fault-injection point (context doc §4):
// every OutSendMessage a node emits passes through here on its way to
// becoming a scheduled EventMessageArrival. A partitioned pair drops the
// message entirely — a real partition loses the packet, it doesn't deliver
// it late after Heal. Beyond that, s.faults' drop/duplicate rates are each
// rolled once against s.rng: a dropped message never gets scheduled at
// all; a duplicated one gets two independently-latency-sampled arrivals,
// which is also what lets a duplicate arrive before or after its original.
func (s *Simulator) scheduleDelivery(from, to int, msg *raft.RaftMessage) {
	if !s.connected(from, to) {
		return
	}
	if s.faults.DropProbability > 0 && s.rng.Float64() < s.faults.DropProbability {
		return
	}

	s.scheduleArrival(from, to, msg)
	if s.faults.DuplicateProbability > 0 && s.rng.Float64() < s.faults.DuplicateProbability {
		s.scheduleArrival(from, to, msg)
	}
}

func (s *Simulator) scheduleArrival(from, to int, msg *raft.RaftMessage) {
	s.schedule(raft.Event{
		At:      s.now + sampleLatency(s.rng),
		NodeID:  to,
		Kind:    raft.EventMessageArrival,
		From:    from,
		Message: msg,
	})
}
