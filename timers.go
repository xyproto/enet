package enet

import (
	"container/heap"
)

type TimerCallback func()
type TimerItem struct {
	weight   int64
	callback TimerCallback
	index    int // heap index
}
type priorityQueue []*TimerItem
type TimerQueue struct{ *priorityQueue }

// sort interface

func (pq priorityQueue) Len() int           { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].weight < pq[j].weight }
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// heap interface

func (pq *priorityQueue) Push(x interface{}) {
	v := x.(*TimerItem)
	v.index = len(*pq)
	*pq = append(*pq, v)
}

func (pq *priorityQueue) Pop() interface{} {
	l := len(*pq)
	v := (*pq)[l-1]
	*pq = (*pq)[:l-1]
	v.index = -1
	return v
}

// timer queue interface
func newTimerQueue() TimerQueue {
	timers := make(priorityQueue, 0)
	heap.Init(&timers)
	return TimerQueue{&timers}
}
func (timers TimerQueue) push(deadline int64, cb TimerCallback) *TimerItem {
	v := &TimerItem{deadline, cb, -1}
	heap.Push(timers, v)
	return v
}

func (timers TimerQueue) pop(now int64) TimerCallback {
	if timers.Len() == 0 {
		return nil
	}
	if (*timers.priorityQueue)[0].weight < now {
		top := heap.Pop(timers).(*TimerItem)
		return top.callback
	}
	return nil
}

func (timers TimerQueue) remove(idx int) {
	assert(idx < timers.Len())
	heap.Remove(timers, idx)
}
