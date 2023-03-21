package util

import (
	"anaflow/src/bgp"
	"fmt"
	"sync"
)

// A concurrent safe queue
// using Interface
type (
	inode struct {
		prev  *inode
		next  *inode
		value interface{}
	}

	ICsqueue struct {
		start  *inode
		end    *inode
		length int
		mu     sync.Mutex
	}
)

func NewICsqueue() *ICsqueue {
	return &ICsqueue{
		start:  nil,
		end:    nil,
		length: 0,
	}
}

func (cq *ICsqueue) CsPush(v interface{}) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	n := &inode{nil, nil, v}
	if cq.length == 0 {
		cq.start = n
		cq.end = n
	} else {
		n.prev = cq.end
		cq.end.next = n
		cq.end = n
	}
	cq.length++
}

func (cq *ICsqueue) Pop() interface{} {
	if cq.length == 0 {
		return nil
	}
	n := cq.start
	if n.next == nil {
		cq.start = nil
	} else {
		cq.start = n.next
		cq.start.prev = nil
	}
	n.next = nil
	n.prev = nil
	cq.length--
	return n.value
}

func (cq *ICsqueue) CsPop() interface{} {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	n := cq.Pop()
	return n
}

// only used for Bgp Update Queue
// return value : 1 continue poping; 0 stop poping
func (cq *ICsqueue) CsPopOverTime(btime uint32) (int, interface{}) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if cq.length == 0 {
		return 0, nil
	}

	// only return Bgpinfo type. Otherwise continue to pop
	info, ok := cq.start.value.(bgp.BgpInfo)
	if !ok {
		fmt.Printf("Error in Csqueue: Cannot convert to a Bgpinfo\n")
		cq.Pop()
		return 1, nil
	}

	if info.New_btime < btime {
		n := cq.Pop()
		return 1, n
	}
	return 0, nil
}

// using generics
type QueueType interface {
	bgp.BgpInfo | bgp.Flow
}
type gnode[T QueueType] struct {
	prev  *gnode[T]
	next  *gnode[T]
	v     T
	utime uint32
}
type GCsqueue[T QueueType] struct {
	start  *gnode[T]
	end    *gnode[T]
	length int
	mu     sync.Mutex
}

func NewGCsqueue[T QueueType]() *GCsqueue[T] {
	return &GCsqueue[T]{
		start:  nil,
		end:    nil,
		length: 0,
	}
}

func (cq *GCsqueue[T]) Push(v T, utime uint32) {
	n := &gnode[T]{nil, nil, v, utime}
	if cq.length == 0 {
		cq.start = n
		cq.end = n
	} else {
		n.prev = cq.end
		cq.end.next = n
		cq.end = n
	}
	cq.length++
}

func (cq *GCsqueue[T]) CsPush(v T, utime uint32) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	cq.Push(v, utime)
}

func (cq *GCsqueue[T]) Pop() (T, bool) {
	if cq.length == 0 {
		var v T
		return v, false
	}
	n := cq.start
	if n.next == nil {
		cq.start = nil
	} else {
		n.next.prev = nil
		cq.start = n.next
	}
	n.next = nil
	n.prev = nil
	cq.length--
	return n.v, true
}

func (cq *GCsqueue[T]) CsPop() (T, bool) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	v, b := cq.Pop()
	return v, b
}

// return value : true continue poping; false stop poping
func (cq *GCsqueue[T]) CsPopOverTime(utime uint32) (T, bool) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if cq.length == 0 {
		var v T
		return v, false
	}

	if cq.start.utime < utime {
		v, b := cq.Pop()
		return v, b
	}
	var v T
	return v, false
}
