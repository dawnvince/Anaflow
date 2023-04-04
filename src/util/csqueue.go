package util

import (
	"anaflow/src/bgp"
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
func (cq *ICsqueue) CsPopOverTime(btime int64) (int, interface{}) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if cq.length == 0 {
		return 0, nil
	}

	// only return Bgpinfo type. Otherwise continue to pop
	info, ok := cq.start.value.(bgp.BgpInfo)
	if !ok {
		// fmt.Printf("Error in Csqueue: Cannot convert to a Bgpinfo\n")
		cq.Pop()
		return 1, nil
	}

	if info.Btime < btime {
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
	utime int64
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

func (cq *GCsqueue[T]) Push(v T, utime int64) {
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

func (cq *GCsqueue[T]) CsPush(v T, utime int64) {
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
func (cq *GCsqueue[T]) CsPopOverTime(utime int64) (T, bool) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if cq.length == 0 {
		var v T
		return v, false
	}
	if cq.start.utime <= utime {
		v, b := cq.Pop()
		return v, b
	}
	var v T
	return v, false
}

// As Flow Queue needs another three pointers, we rewrite the flow queue
type flownode struct {
	next  *flownode
	v     bgp.Flow
	utime int64
}

type FlowCsqueue struct {
	pri_s_t    int64
	pri_e_t    int64
	post_s_t   int64
	post_e_t   int64
	length     int
	pri_start  *flownode // = q_start
	pri_end    *flownode
	post_start *flownode
	post_end   *flownode
	q_end      *flownode
	mu         sync.Mutex
}

func NewFlowCsqueue() *FlowCsqueue {
	fq := new(FlowCsqueue)
	fq.pri_start = &flownode{next: nil, utime: int64((^uint64(0)) >> 1)}
	fq.pri_end = fq.pri_start
	fq.post_end = fq.pri_start
	fq.post_start = fq.pri_start
	fq.q_end = fq.pri_start
	fq.length = 0
	return fq
}

func (cq *FlowCsqueue) Push(v bgp.Flow, utime int64) {

	n := &flownode{nil, v, utime}
	if cq.length == 0 {
		cq.pri_start.next = n
		cq.q_end = n
	} else {
		cq.q_end.next = n
		cq.q_end = n
	}
	cq.length++
}

func (cq *FlowCsqueue) CsPush(v bgp.Flow, utime int64) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	cq.Push(v, utime)
}

// save one local variable copy
func (cq *FlowCsqueue) PopWoReturn() {
	n := cq.pri_start.next
	if n.next == nil {
		cq.pri_start.next = nil
		cq.pri_end = cq.pri_start
		cq.post_start = cq.pri_start
		cq.post_end = cq.pri_start
		cq.q_end = cq.pri_start
	} else {
		cq.pri_start.next = n.next
	}
	n.next = nil
	cq.length--
}

func (cq *FlowCsqueue) CsPopPriStartOverTime(v_ptr *bgp.Flow) bool {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if cq.length == 0 {
		return false
	}
	if cq.pri_start.next.utime <= cq.pri_s_t {
		*v_ptr = cq.pri_start.next.v
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
	if cq.pri_end.next == nil {
		return false
	}
	if cq.pri_end.next.utime <= cq.pri_e_t {
		*v_ptr = cq.pri_end.next.v
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
	if cq.post_start.next == nil {
		return false
	}
	if cq.post_start.next.utime <= cq.post_s_t {
		*v_ptr = cq.post_start.next.v
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
	if cq.post_end.next == nil {
		return false
	}
	if cq.post_end.next.utime <= cq.post_e_t {
		*v_ptr = cq.post_end.next.v
		cq.post_end = cq.post_end.next
		return true
	}
	return false
}

func (cq *FlowCsqueue) ModifyTime(utime int64, delay int64, agetime int64, syncdevi int64) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	cq.pri_s_t = utime - delay - 2*agetime
	cq.pri_e_t = utime - delay - agetime - syncdevi
	cq.post_s_t = utime - delay - agetime + syncdevi
	cq.post_e_t = utime - delay
}

func (cq *FlowCsqueue) GetLength() int {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	l := cq.length
	return l
}
