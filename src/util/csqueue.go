package util

import (
	"anaflow/src/bgp"
	"errors"
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
	bgp.BgpInfo | bgp.Flow | uint32
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
		cq.end = nil
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

// As Flow Queue needs another three pointers, we rewrite the flow queue
type flownode struct {
	prev  *flownode
	next  *flownode
	v     bgp.Flow
	utime uint32
}

type FlowCsqueue struct {
	pri_s_t    uint32
	pri_e_t    uint32
	post_s_t   uint32
	post_e_t   uint32
	length     int
	pri_start  *flownode // = q_start
	pri_end    *flownode
	post_start *flownode
	post_end   *flownode
	q_end      *flownode
	mu         sync.Mutex
}

func NewFlowCsqueue() *FlowCsqueue {
	return &FlowCsqueue{
		pri_start:  nil,
		pri_end:    nil,
		post_start: nil,
		post_end:   nil,
		q_end:      nil,
		length:     0,
	}
}

func (cq *FlowCsqueue) Push(v bgp.Flow, utime uint32) {

	n := &flownode{nil, nil, v, utime}
	if cq.length == 0 {
		cq.pri_start = n
		cq.pri_end = n
		cq.post_start = n
		cq.post_end = n
		cq.q_end = n
	} else {
		n.prev = cq.q_end
		cq.q_end.next = n
		cq.q_end = n
	}
	cq.length++
}

func (cq *FlowCsqueue) CsPush(v bgp.Flow, utime uint32) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	cq.Push(v, utime)
}

// save one local variable copy
func (cq *FlowCsqueue) PopWoReturn() {
	if cq.length == 0 {
		return
	}
	n := cq.pri_start
	if n.next == nil {
		if cq.length != 1 {
			PanicError(errors.New("func PopWoReturn: "), "n.next = NIL while length != 1\n")
		}
		cq.pri_start = nil
		cq.q_end = nil
	} else {
		n.next.prev = nil
		cq.pri_start = n.next
	}
	n.next = nil
	n.prev = nil
	cq.length--
}

func (cq *FlowCsqueue) CsPopPriStartOverTime(v_ptr *bgp.Flow) bool {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if cq.length == 0 {
		return false
	}
	if cq.pri_start.utime <= cq.pri_s_t {
		*v_ptr = cq.pri_start.v
		cq.PopWoReturn()
		return true
	}
	return false
}

func (cq *FlowCsqueue) CsOnePriEndOvertime(v_ptr *bgp.Flow) bool {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if cq.length == 0 {
		return false
	}
	if cq.pri_end.utime <= cq.pri_e_t {
		*v_ptr = cq.pri_end.v
		cq.pri_end = cq.pri_end.next
		return true
	}
	return false
}

func (cq *FlowCsqueue) CsOnePostStartOvertime(v_ptr *bgp.Flow) bool {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if cq.length == 0 {
		return false
	}
	if cq.post_start.utime <= cq.post_s_t {
		*v_ptr = cq.post_start.v
		cq.post_start = cq.post_start.next
		return true
	}
	return false
}

func (cq *FlowCsqueue) CsOnePostEndOvertime(v_ptr *bgp.Flow) bool {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if cq.length == 0 {
		return false
	}
	if cq.post_end.utime <= cq.post_e_t {
		*v_ptr = cq.post_end.v
		cq.post_end = cq.post_end.next
		return true
	}
	return false
}

func (cq *FlowCsqueue) ModifyTime(utime uint32, delay uint32, agetime uint32, syncdevi uint32) {
	cq.pri_s_t = utime - delay - 2*agetime - 2*syncdevi
	cq.pri_e_t = cq.pri_s_t + agetime
	cq.post_s_t = utime - delay - agetime
	cq.post_e_t = utime - delay
}
