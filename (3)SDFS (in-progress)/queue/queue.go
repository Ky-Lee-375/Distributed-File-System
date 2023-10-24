package queue

import (
	"container/list"
	"fmt"
)

// QueueElement represents an element in the queue containing destination and request body.
type QueueElement struct {
	Dest   string
	RBody  string
	OpType int
}

// Queue is a queue data structure.
type Queue struct {
	l *list.List
}

// NewQueue creates a new Queue.
func NewQueue() *Queue {
	return &Queue{l: list.New()}
}

// Enqueue adds a QueueElement to the end of the queue.
func (q *Queue) Enqueue(dest string, rBody string, optype int) {
	element := QueueElement{
		Dest:   dest,
		RBody:  rBody,
		OpType: optype,
	}
	q.l.PushBack(element)
}

// Dequeue removes and returns the first element from the queue.
func (q *Queue) Dequeue() (QueueElement, bool) {
	if q.l.Len() == 0 {
		return QueueElement{}, false
	}
	e := q.l.Front()
	q.l.Remove(e)
	return e.Value.(QueueElement), true
}

// IsEmpty checks if the queue is empty.
func (q *Queue) IsEmpty() bool {
	return q.l.Len() == 0
}

func (q *Queue) Print() {
	fmt.Println("Queue contents:")
	for e := q.l.Front(); e != nil; e = e.Next() {
		element := e.Value.(QueueElement)
		fmt.Printf("Dest: %s, RBody: %s\n", element.Dest, element.RBody)
	}
}

// Peek returns the first element from the queue without removing it.
func (q *Queue) Peek() (QueueElement, bool) {
	if q.l.Len() == 0 {
		return QueueElement{}, false
	}
	e := q.l.Front()
	return e.Value.(QueueElement), true
}