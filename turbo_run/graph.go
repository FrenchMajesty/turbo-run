package turbo_run

import (
	"sync"

	"github.com/google/uuid"
)

type Graph struct {
	nodes          map[uuid.UUID]*WorkNode
	children       map[uuid.UUID][]uuid.UUID
	indegree       map[uuid.UUID]int
	mutex          sync.RWMutex
	readyNodesChan chan *WorkNode
}

func NewGraph() *Graph {
	return &Graph{
		nodes:          make(map[uuid.UUID]*WorkNode),
		children:       make(map[uuid.UUID][]uuid.UUID),
		indegree:       make(map[uuid.UUID]int),
		readyNodesChan: make(chan *WorkNode, 1000),
	}
}

func (g *Graph) Size() int {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return len(g.nodes)
}

// Add adds a node to the graph and updates the dependencies
func (g *Graph) Add(node *WorkNode, dependencies []uuid.UUID) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.nodes[node.ID] = node
	g.indegree[node.ID] = len(dependencies)
	g.children[node.ID] = []uuid.UUID{}

	for _, dependency := range dependencies {
		g.children[dependency] = append(g.children[dependency], node.ID)
	}

	if len(dependencies) == 0 {
		g.readyNodesChan <- node
	}
}

// Remove removes a node from the graph and updates the indegree of the children
func (g *Graph) Remove(uuid uuid.UUID) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	delete(g.nodes, uuid)
	delete(g.indegree, uuid)

	for _, child := range g.children[uuid] {
		g.indegree[child]--

		if g.indegree[child] == 0 {
			g.readyNodesChan <- g.nodes[child]
		}
	}

	delete(g.children, uuid)
}

// GetNodesWithNoDependencies returns the nodes with no dependencies
func (g *Graph) GetNodesWithNoDependencies() []*WorkNode {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	entrypoints := []*WorkNode{}
	for _, node := range g.nodes {
		if g.indegree[node.ID] == 0 {
			entrypoints = append(entrypoints, node)
		}
	}

	return entrypoints
}
