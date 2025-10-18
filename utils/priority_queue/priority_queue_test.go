package priority_queue

import (
	"testing"
)

func TestMaxPriorityQueue(t *testing.T) {
	pq := NewMaxPriorityQueue[string]()

	items := []struct {
		value    string
		priority int
	}{
		{"low", 1},
		{"high", 10},
		{"medium", 5},
		{"highest", 15},
	}

	for _, item := range items {
		queueItem := &QueueItem[string]{
			Item:     item.value,
			Priority: item.priority,
		}
		size := pq.Push(queueItem)
		if size == 0 {
			t.Error("Expected size > 0 after push")
		}
	}

	if pq.Size() != 4 {
		t.Errorf("Expected size 4, got %d", pq.Size())
	}

	expected := []string{"highest", "high", "medium", "low"}
	for i, expectedValue := range expected {
		value, size := pq.Pop()
		if value != expectedValue {
			t.Errorf("Pop %d: expected %s, got %s", i, expectedValue, value)
		}
		if size != len(expected)-i-1 {
			t.Errorf("Pop %d: expected size %d, got %d", i, len(expected)-i-1, size)
		}
	}

	if pq.Size() != 0 {
		t.Errorf("Expected empty queue, got size %d", pq.Size())
	}
}

func TestMinPriorityQueue(t *testing.T) {
	pq := NewMinPriorityQueue[string]()

	items := []struct {
		value    string
		priority int
	}{
		{"low", 1},
		{"high", 10},
		{"medium", 5},
		{"highest", 15},
	}

	for _, item := range items {
		queueItem := &QueueItem[string]{
			Item:     item.value,
			Priority: item.priority,
		}
		pq.Push(queueItem)
	}

	expected := []string{"low", "medium", "high", "highest"}
	for i, expectedValue := range expected {
		value, _ := pq.Pop()
		if value != expectedValue {
			t.Errorf("Pop %d: expected %s, got %s", i, expectedValue, value)
		}
	}
}

func TestNewPriorityQueue_BackwardCompatibility(t *testing.T) {
	pq := NewPriorityQueue[int]()

	items := []int{1, 10, 5, 15}
	for _, priority := range items {
		queueItem := &QueueItem[int]{
			Item:     priority,
			Priority: priority,
		}
		pq.Push(queueItem)
	}

	expected := []int{15, 10, 5, 1}
	for i, expectedValue := range expected {
		value, _ := pq.Pop()
		if value != expectedValue {
			t.Errorf("Pop %d: expected %d, got %d", i, expectedValue, value)
		}
	}
}

func TestPriorityQueue_EmptyQueue(t *testing.T) {
	pq := NewMaxPriorityQueue[string]()

	if pq.Size() != 0 {
		t.Errorf("Expected empty queue size 0, got %d", pq.Size())
	}
}

func TestPriorityQueue_SingleItem(t *testing.T) {
	pq := NewMaxPriorityQueue[string]()

	queueItem := &QueueItem[string]{
		Item:     "only",
		Priority: 42,
	}

	size := pq.Push(queueItem)
	if size != 1 {
		t.Errorf("Expected size 1 after push, got %d", size)
	}

	value, size := pq.Pop()
	if value != "only" {
		t.Errorf("Expected 'only', got %s", value)
	}
	if size != 0 {
		t.Errorf("Expected size 0 after pop, got %d", size)
	}
}

func TestPriorityQueue_SamePriority(t *testing.T) {
	pq := NewMaxPriorityQueue[string]()

	items := []string{"first", "second", "third"}
	for _, item := range items {
		queueItem := &QueueItem[string]{
			Item:     item,
			Priority: 5,
		}
		pq.Push(queueItem)
	}

	results := make([]string, 0, len(items))
	for i := 0; i < len(items); i++ {
		value, _ := pq.Pop()
		results = append(results, value)
	}

	if len(results) != len(items) {
		t.Errorf("Expected %d items, got %d", len(items), len(results))
	}
}

func TestPriorityQueue_DifferentTypes(t *testing.T) {
	pq := NewMaxPriorityQueue[int]()

	items := []struct {
		value    int
		priority int
	}{
		{100, 1},
		{200, 3},
		{300, 2},
	}

	for _, item := range items {
		queueItem := &QueueItem[int]{
			Item:     item.value,
			Priority: item.priority,
		}
		pq.Push(queueItem)
	}

	first, _ := pq.Pop()
	if first != 200 {
		t.Errorf("Expected 200 (priority 3), got %d", first)
	}

	second, _ := pq.Pop()
	if second != 300 {
		t.Errorf("Expected 300 (priority 2), got %d", second)
	}

	third, _ := pq.Pop()
	if third != 100 {
		t.Errorf("Expected 100 (priority 1), got %d", third)
	}
}