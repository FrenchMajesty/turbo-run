package priority_queue

import (
	"container/heap"
	"sync"
)

// QueueItem is a wrapper around the item to be stored in the priority queue.
type QueueItem[T any] struct {
	Item     T
	Priority int
	index    int
}

// PriorityQueue is a thread-safe priority queue that wraps the heapQueue implementation.
// It is used to store items with a priority and retrieve them in the order of their priority.
type PriorityQueue[T any] struct {
	queue *heapQueue[T]
	mutex sync.Mutex
}

// NewMaxPriorityQueue creates a max heap (higher priority values come first)
func NewMaxPriorityQueue[T any]() *PriorityQueue[T] {
	priorityQueue := &PriorityQueue[T]{
		queue: &heapQueue[T]{
			items: make([]*QueueItem[T], 0),
			less:  func(i, j int) bool { return i > j }, // max heap
		},
	}

	heap.Init(priorityQueue.queue)
	return priorityQueue
}

// NewMinPriorityQueue creates a min heap (lower priority values come first)
func NewMinPriorityQueue[T any]() *PriorityQueue[T] {
	priorityQueue := &PriorityQueue[T]{
		queue: &heapQueue[T]{
			items: make([]*QueueItem[T], 0),
			less:  func(i, j int) bool { return i < j }, // min heap
		},
	}

	heap.Init(priorityQueue.queue)
	return priorityQueue
}

// NewPriorityQueue creates a max heap (maintains backward compatibility)
func NewPriorityQueue[T any]() *PriorityQueue[T] {
	return NewMaxPriorityQueue[T]()
}

// Push adds a WorkNode to the PriorityQueue
func (pq *PriorityQueue[T]) Push(item *QueueItem[T]) int {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	heap.Push(pq.queue, item)
	return len(pq.queue.items)
}

// Pop removes and returns the item with the highest priority
func (pq *PriorityQueue[T]) Pop() (T, int) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	item := heap.Pop(pq.queue).(*QueueItem[T])
	return item.Item, len(pq.queue.items)
}

// Size returns the number of items in the priority queue
func (pq *PriorityQueue[T]) Size() int {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	return len(pq.queue.items)
}

// GetSnapshot returns a copy of all items in the priority queue in priority order
// without modifying the queue. Items are returned in the order they would be popped.
func (pq *PriorityQueue[T]) GetSnapshot() []T {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	items := make([]T, len(pq.queue.items))
	for i, item := range pq.queue.items {
		items[i] = item.Item
	}
	return items
}
