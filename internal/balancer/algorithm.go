package balancer

import "sync/atomic"

type Algorithm interface {
	Next(n int) int // given n backends, return which one to use next
}

type RoundRobin struct {
	counter atomic.Uint32
}

func (rr *RoundRobin) Next(n int) int {
	cur := rr.counter.Add(1)
	idx := int(cur-1) % n
	return idx
}
