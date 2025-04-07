package utils

import "sync"

type IDGenerator struct {
	mu sync.Mutex
	nextID int
}

func NewIDGenerator (initialMaxID int) *IDGenerator{
	return &IDGenerator{
		nextID: initialMaxID + 1,
	}
}

func (g *IDGenerator) NextID() int{
	g.mu.Lock()
	defer g.mu.Unlock()
	id := g.nextID
	g.nextID++
	return id
}

func (g *IDGenerator) UpdateGenerator(maxID int){
	g.mu.Lock()
	defer g.mu.Unlock()
	if maxID >= g.nextID {
		g.nextID = maxID + 1
	}
}