package turbo_run

import (
	"math"
	"sync"

	"github.com/google/uuid"
)

type Graph struct {
	nodes          map[uuid.UUID]*WorkNode
	children       map[uuid.UUID][]uuid.UUID
	indegree       map[uuid.UUID]int
	mutex          sync.RWMutex
	readyNodesChan chan *WorkNode
	size           int
}

// NewGraph creates a new graph with a configurable size
func NewGraph(size int) *Graph {
	bufferSize := int(math.Min(10_000, float64(size)/3)) // at most 10K buffer size
	if bufferSize <= 0 {
		bufferSize = 1000 // Default buffer size on unbounded graphs
	}

	return &Graph{
		nodes:          make(map[uuid.UUID]*WorkNode, size),
		children:       make(map[uuid.UUID][]uuid.UUID, size),
		indegree:       make(map[uuid.UUID]int, size),
		readyNodesChan: make(chan *WorkNode, bufferSize),
		size:           size,
	}
}

// Size returns the number of nodes in the graph
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
	g.remove(uuid, true)
}

// remove is a helper that removes a node from the graph and updates the indegree of the children. Must be called with mutex already held.
func (g *Graph) remove(uuid uuid.UUID, emitReady bool) {
	delete(g.nodes, uuid)
	delete(g.indegree, uuid)

	for _, child := range g.children[uuid] {
		g.indegree[child]--

		if g.indegree[child] == 0 && emitReady {
			g.readyNodesChan <- g.nodes[child]
		}
	}

	delete(g.children, uuid)
}

// RemoveSubtree removes a node and all of its descendants from the graph recursively.
// This is used when a failure should propagate down the dependency tree.
// Returns the list of all removed node IDs (including the root).
func (g *Graph) RemoveSubtree(nodeID uuid.UUID) []uuid.UUID {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	removedIDs := []uuid.UUID{}
	g.removeSubtreeRecursive(nodeID, &removedIDs)
	// reverse list to have root at index 0
	for i, j := 0, len(removedIDs)-1; i < j; i, j = i+1, j-1 {
		removedIDs[i], removedIDs[j] = removedIDs[j], removedIDs[i]
	}

	return removedIDs
}

// removeSubtreeRecursive is a helper that recursively removes a node and its descendants
// Must be called with mutex already held
func (g *Graph) removeSubtreeRecursive(nodeID uuid.UUID, removedIDs *[]uuid.UUID) {
	// If node doesn't exist, nothing to do
	if _, exists := g.nodes[nodeID]; !exists {
		return
	}

	// First, recursively remove all children
	childrenToRemove := make([]uuid.UUID, len(g.children[nodeID]))
	copy(childrenToRemove, g.children[nodeID])

	for _, childID := range childrenToRemove {
		g.removeSubtreeRecursive(childID, removedIDs)
	}

	g.remove(nodeID, false)
	*removedIDs = append(*removedIDs, nodeID)
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

// Clear removes all nodes from the graph and drains the ready channel.
// Returns the list of all removed node IDs.
func (g *Graph) Clear() []uuid.UUID {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Collect all node IDs
	removedIDs := make([]uuid.UUID, 0, len(g.nodes))
	for nodeID := range g.nodes {
		removedIDs = append(removedIDs, nodeID)
	}

	// Clear all maps
	g.nodes = make(map[uuid.UUID]*WorkNode, g.size)
	g.children = make(map[uuid.UUID][]uuid.UUID, g.size)
	g.indegree = make(map[uuid.UUID]int, g.size)

	// Drain the ready channel (non-blocking)
	for {
		select {
		case <-g.readyNodesChan:
			// Drained one item
		default:
			// Channel is empty
			return removedIDs
		}
	}
}
