package main

// TODO:
// - Given a list of backends, pick the next one.

// A balancer needs:
// - A list of backends
// - Which one to pick next
// A balancer:
// - Returns the next backend
// (- Eventually skip dead backends)

type Balancer struct {
	instances []int // List of backend ports
	current   int   // current port index
}

func (b *Balancer) GetPort() int {
}
