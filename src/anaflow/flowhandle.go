package anaflow

import (
	"anaflow/src/bgp"
	"anaflow/src/util"
	"errors"
)

// Global shared structures. Need concurrent safe methods.
var Updata_queue *util.GCsqueue[bgp.BgpInfo]
var Flow_queue *util.FlowCsqueue

// Local structure without concurrent problems.

// Given a route entry, find the list of dst_ip using this route.
// Nesting structure enables O(1) insertion/deletion time for each Dst_ip
var priRoute2Dst map[uint32](map[uint32]uint64)  // PriRD
var priDst2Route map[uint32][]bgp.IpInfo         // PriDR
var postRoute2Dst map[uint32](map[uint32]uint64) // PostDR
var postDst2Route map[uint32][]bgp.IpInfo        // PostRD

const INITVOLUME = 65536

func init() {
	Updata_queue = util.NewGCsqueue[bgp.BgpInfo]()
	Flow_queue = util.NewFlowCsqueue()
	priRoute2Dst = make(map[uint32](map[uint32]uint64), INITVOLUME)
	priDst2Route = make(map[uint32][]bgp.IpInfo, INITVOLUME)
	postRoute2Dst = make(map[uint32](map[uint32]uint64), INITVOLUME)
	postDst2Route = make(map[uint32][]bgp.IpInfo, INITVOLUME)
}

func addFlow2Pri(v_ptr *bgp.Flow) {
	rp := v_ptr.Route >> (32 - v_ptr.Prefix)

	// add flow to priRoute2Dst
	dst_list, ok_out := priRoute2Dst[rp]
	if ok_out {
		_, ok_in := dst_list[v_ptr.Dst_ip]
		if ok_in {
			dst_list[v_ptr.Dst_ip] += v_ptr.Size
		} else {
			dst_list[v_ptr.Dst_ip] = v_ptr.Size
		}
	} else {
		priRoute2Dst[rp] = map[uint32]uint64{
			v_ptr.Dst_ip: v_ptr.Size,
		}
	}

	// add flow to postDst2Route
	route_q, ok_q := postDst2Route[v_ptr.Dst_ip]
	if ok_q {
		if route_q[len(route_q)-1].RoutePrefix == rp {
			postDst2Route[v_ptr.Dst_ip][len(route_q)-1].Size += v_ptr.Size
		} else {
			postDst2Route[v_ptr.Dst_ip] = append(postDst2Route[v_ptr.Dst_ip], bgp.IpInfo{
				RoutePrefix: rp,
				Size:        v_ptr.Size,
			})
		}
	} else {
		postDst2Route[v_ptr.Dst_ip] = []bgp.IpInfo{
			{
				RoutePrefix: rp,
				Size:        v_ptr.Size,
			},
		}
	}
}

func delFlowFromPri(v_ptr *bgp.Flow) {
	rp := v_ptr.Route >> (32 - v_ptr.Prefix)

	// delete flow from priRoute2Dst
	priRoute2Dst[rp][v_ptr.Dst_ip] -= v_ptr.Size
	if priRoute2Dst[rp][v_ptr.Dst_ip] <= 0 {
		if len(priRoute2Dst[rp]) <= 0 {
			delete(priRoute2Dst, rp)
		} else {
			delete(priRoute2Dst[rp], v_ptr.Dst_ip)
		}
	}

	// delete flow from postDst2Route
	if postDst2Route[v_ptr.Dst_ip][0].RoutePrefix != rp {
		util.PanicError(errors.New("func delFlowFromPri: "), "postDst2Route[v_ptr.Dst_ip][0].RoutePrefix != rp\n")
	}
	postDst2Route[v_ptr.Dst_ip][0].Size -= v_ptr.Size
	if postDst2Route[v_ptr.Dst_ip][0].Size <= 0 {
		if len(postDst2Route[v_ptr.Dst_ip]) == 1 {
			delete(postDst2Route, v_ptr.Dst_ip)
		} else {
			postDst2Route[v_ptr.Dst_ip] = postDst2Route[v_ptr.Dst_ip][1:]
		}
	}
}

func addFlow2Post(v_ptr *bgp.Flow) {
	rp := v_ptr.Route >> (32 - v_ptr.Prefix)

	// add flow to postRoute2Dst
	dst_list, ok_out := postRoute2Dst[rp]
	if ok_out {
		_, ok_in := dst_list[v_ptr.Dst_ip]
		if ok_in {
			dst_list[v_ptr.Dst_ip] += v_ptr.Size
		} else {
			dst_list[v_ptr.Dst_ip] = v_ptr.Size
		}
	} else {
		postRoute2Dst[rp] = map[uint32]uint64{
			v_ptr.Dst_ip: v_ptr.Size,
		}
	}

	// add flow to priDst2Route
	route_q, ok_q := priDst2Route[v_ptr.Dst_ip]
	if ok_q {
		if route_q[len(route_q)-1].RoutePrefix == rp {
			priDst2Route[v_ptr.Dst_ip][len(route_q)-1].Size += v_ptr.Size
		} else {
			priDst2Route[v_ptr.Dst_ip] = append(priDst2Route[v_ptr.Dst_ip], bgp.IpInfo{
				RoutePrefix: rp,
				Size:        v_ptr.Size,
			})
		}
	} else {
		priDst2Route[v_ptr.Dst_ip] = []bgp.IpInfo{
			{
				RoutePrefix: rp,
				Size:        v_ptr.Size,
			},
		}
	}
}

func delFlowFromPost(v_ptr *bgp.Flow) {
	rp := v_ptr.Route >> (32 - v_ptr.Prefix)

	// delete flow from postRoute2Dst
	postRoute2Dst[rp][v_ptr.Dst_ip] -= v_ptr.Size
	if postRoute2Dst[rp][v_ptr.Dst_ip] <= 0 {
		if len(postRoute2Dst[rp]) <= 0 {
			delete(postRoute2Dst, rp)
		} else {
			delete(postRoute2Dst[rp], v_ptr.Dst_ip)
		}
	}

	// delete flow from priDst2Route
	if priDst2Route[v_ptr.Dst_ip][0].RoutePrefix != rp {
		util.PanicError(errors.New("func delFlowFromPri: "), "priDst2Route[v_ptr.Dst_ip][0].RoutePrefix != rp\n")
	}
	priDst2Route[v_ptr.Dst_ip][0].Size -= v_ptr.Size
	if priDst2Route[v_ptr.Dst_ip][0].Size <= 0 {
		if len(priDst2Route[v_ptr.Dst_ip]) == 1 {
			delete(priDst2Route, v_ptr.Dst_ip)
		} else {
			priDst2Route[v_ptr.Dst_ip] = priDst2Route[v_ptr.Dst_ip][1:]
		}
	}
}

func AddFlow2Q(flow bgp.Flow) {
	// End t to modify
	Flow_queue.CsPush(flow, flow.End_t)
}

/*
Exec GivenCurrentTime every second.
*/

// PRISTART       BGPUPDATE        POSTEND    UTIME
//    |--(agetime)--|-|-|--(agetime)--|--delay--|
//                   | |
//              sync deviation

func GivenCurrentTime(utime uint32, delay uint32, agetime uint32, syncdevi uint32) {
	Flow_queue.ModifyTime(utime, delay, agetime, syncdevi)
	var v_ptr *bgp.Flow
	var flag bool

	// ADD flows at time (BGPUPDATE - syncdevi) to PriMaps
	for flag = Flow_queue.CsOnePriEndOvertime(v_ptr); flag; flag = Flow_queue.CsOnePriEndOvertime(v_ptr) {
		addFlow2Pri(v_ptr)
	}

	// Delete outdated(before delay+2*agetime+2*syncdevi) entry in PriRoute2Dst and PriDst2Route
	for flag = Flow_queue.CsPopPriStartOverTime(v_ptr); flag; flag = Flow_queue.CsPopPriStartOverTime(v_ptr) {
		delFlowFromPri(v_ptr)
	}

	// ADD flows at time POSTEND to PriMaps
	for flag = Flow_queue.CsOnePostEndOvertime(v_ptr); flag; flag = Flow_queue.CsOnePostEndOvertime(v_ptr) {
		addFlow2Post(v_ptr)
	}

	// Delete outdated(before delay+agetime) entry in PriRoute2Dst and PriDst2Route
	for flag = Flow_queue.CsOnePostStartOvertime(v_ptr); flag; flag = Flow_queue.CsOnePostStartOvertime(v_ptr) {
		delFlowFromPost(v_ptr)
	}

	// For each update BU at this time
	// 		Calculate Update Effect scope
	// 明确一下作用域重合的问题
	// If type is add: get dst_IP list that use BU after BGPUPDATE.
	//		For each IP, if its route queue head is quite BU, we think this IP is affected by BU
	//		Find this IP in
}

func GivenUpdate(bu *bgp.BgpInfo)

// func AddRteBefore(r2d map[uint32]*util.Csqueue, d2r map[uint32]*util.Csqueue)
