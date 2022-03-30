package enet

import (
	"container/heap"
)

type EnetTimerCallback func()
type EnetTimerItem struct {
	weight   int64
	callback EnetTimerCallback
	index    int // heap index
}
type priorityQueue []*EnetTimerItem
type EnetTimerQueue struct{ *priorityQueue }

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
	v := x.(*EnetTimerItem)
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
func newEnetTimerQueue() EnetTimerQueue {
	timers := make(priorityQueue, 0)
	heap.Init(&timers)
	return EnetTimerQueue{&timers}
}
func (timers EnetTimerQueue) push(deadline int64, cb EnetTimerCallback) *EnetTimerItem {
	v := &EnetTimerItem{deadline, cb, -1}
	heap.Push(timers, v)
	return v
}

func (timers EnetTimerQueue) pop(now int64) EnetTimerCallback {
	if timers.Len() == 0 {
		return nil
	}
	if (*timers.priorityQueue)[0].weight < now {
		top := heap.Pop(timers).(*EnetTimerItem)
		return top.callback
	}
	return nil
}

func (timers EnetTimerQueue) remove(idx int) {
	assert(idx < timers.Len())
	heap.Remove(timers, idx)
}
