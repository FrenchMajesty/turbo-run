package priority_queue

import "container/heap"

/*
*
// heapQueue is an internal implementation of Go's heap.Interface that wraps
// QueueItem elements to provide priority queue functionality with custom comparison logic.
*/
type heapQueue[T any] struct {
	items []*QueueItem[T]
	less  func(i, j int) bool
}

var _ heap.Interface = &heapQueue[any]{}

func (pq heapQueue[T]) Len() int { return len(pq.items) }
func (pq heapQueue[T]) Less(i, j int) bool {
	return pq.less(pq.items[i].Priority, pq.items[j].Priority)
}
func (pq heapQueue[T]) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].index = i
	pq.items[j].index = j
}

// Push adds a node to the queue with the given priority
func (pq *heapQueue[T]) Push(item any) {
	n := len(pq.items)
	itemI := item.(*QueueItem[T])
	itemI.index = n
	pq.items = append(pq.items, itemI)
}

// Pop removes and returns the node with the highest priority
func (pq *heapQueue[T]) Pop() any {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // don't stop the GC from reclaiming the item eventually
	item.index = -1 // for safety
	pq.items = old[0 : n-1]
	return item
}
