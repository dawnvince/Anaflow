package anaflow

import (
	"anaflow/src/bgp"
	"anaflow/src/util"
	"errors"
	"fmt"
)

// Global shared structures. Need concurrent safe methods.
var Updata_queue *util.GCsqueue[bgp.BgpInfo]
var Flow_queue *util.FlowCsqueue

// Local structure without concurrent problems.

// Given a route entry, find the list of dst_ip using this route.
// Nesting structure enables O(1) insertion/deletion time for each Dst_ip
var priRoute2Dst map[uint64](map[uint32]uint64)  // PriRD
var priDst2Route map[uint32][]bgp.IpInfo         // PriDR
var postRoute2Dst map[uint64](map[uint32]uint64) // PostDR
var postDst2Route map[uint32][]bgp.IpInfo        // PostRD

const INITVOLUME = 524288

func init() {
	Updata_queue = util.NewGCsqueue[bgp.BgpInfo]()
	Flow_queue = util.NewFlowCsqueue()
	priRoute2Dst = make(map[uint64](map[uint32]uint64), INITVOLUME)
	priDst2Route = make(map[uint32][]bgp.IpInfo, INITVOLUME)
	postRoute2Dst = make(map[uint64](map[uint32]uint64), INITVOLUME)
	postDst2Route = make(map[uint32][]bgp.IpInfo, INITVOLUME)
}

func addFlow2Pri(v_ptr *bgp.Flow) {
	rp := uint64(v_ptr.Route)>>(32-v_ptr.Prefix)<<(40-v_ptr.Prefix) + uint64(v_ptr.Prefix)

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
	// fmt.Printf("\033[41;37mCurTime: %d\033[0m\n", time.Now().Unix())
	// fmt.Printf("\033[31mpriRoute2Dst \033[0mis %+v\n\033[31mpriDst2Route \033[0mis %+v\n", priRoute2Dst[rp], priDst2Route[v_ptr.Dst_ip])
}

func delFlowFromPri(v_ptr *bgp.Flow) {
	rp := uint64(v_ptr.Route)>>(32-v_ptr.Prefix)<<(40-v_ptr.Prefix) + uint64(v_ptr.Prefix)

	// delete flow from priRoute2Dst
	priRoute2Dst[rp][v_ptr.Dst_ip] -= v_ptr.Size
	if priRoute2Dst[rp][v_ptr.Dst_ip] <= 0 {
		if len(priRoute2Dst[rp]) <= 1 {
			delete(priRoute2Dst, rp)
		} else {
			delete(priRoute2Dst[rp], v_ptr.Dst_ip)
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
	// fmt.Printf("\033[44;37mCurTime: %d\033[0m\n", time.Now().Unix())
}

func addFlow2Post(v_ptr *bgp.Flow) {
	rp := uint64(v_ptr.Route)>>(32-v_ptr.Prefix)<<(40-v_ptr.Prefix) + uint64(v_ptr.Prefix)

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

	// fmt.Printf("\033[42;37mCurTime: %d\033[0m\n", time.Now().Unix())
	// fmt.Printf("\033[32mpostRoute2Dst \033[0mis %+v\n\033[32mpostDst2Route \033[0mis %+v\n", postRoute2Dst[rp], postDst2Route[v_ptr.Dst_ip])
}

func delFlowFromPost(v_ptr *bgp.Flow) {
	rp := uint64(v_ptr.Route)>>(32-v_ptr.Prefix)<<(40-v_ptr.Prefix) + uint64(v_ptr.Prefix)

	// delete flow from postRoute2Dst
	postRoute2Dst[rp][v_ptr.Dst_ip] -= v_ptr.Size
	if postRoute2Dst[rp][v_ptr.Dst_ip] <= 0 {
		if len(postRoute2Dst[rp]) <= 1 {
			delete(postRoute2Dst, rp)
		} else {
			delete(postRoute2Dst[rp], v_ptr.Dst_ip)
		}
	}

	// delete flow from postDst2Route
	if postDst2Route[v_ptr.Dst_ip][0].RoutePrefix != rp {
		util.PanicError(errors.New("func delFlowFrompost: "), "postDst2Route[v_ptr.Dst_ip][0].RoutePrefix != rp\n")
	}
	postDst2Route[v_ptr.Dst_ip][0].Size -= v_ptr.Size
	if postDst2Route[v_ptr.Dst_ip][0].Size <= 0 {
		if len(postDst2Route[v_ptr.Dst_ip]) == 1 {
			delete(postDst2Route, v_ptr.Dst_ip)
		} else {
			postDst2Route[v_ptr.Dst_ip] = postDst2Route[v_ptr.Dst_ip][1:]
		}
	}

	// fmt.Printf("\033[43;37mCurTime: %d\033[0m\n", time.Now().Unix())
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

func GivenCurrentTime(utime int64, delay int64, agetime int64, syncdevi int64) {
	Flow_queue.ModifyTime(utime, delay, agetime, syncdevi)
	v_ptr := new(bgp.Flow)
	var flag bool

	// ADD flows at time POSTEND to PriMaps
	for flag = Flow_queue.CsOnePostEndOvertime(v_ptr); flag; flag = Flow_queue.CsOnePostEndOvertime(v_ptr) {
		addFlow2Post(v_ptr)
	}

	// Delete outdated(before delay+agetime) entry in PriRoute2Dst and PriDst2Route
	for flag = Flow_queue.CsOnePostStartOvertime(v_ptr); flag; flag = Flow_queue.CsOnePostStartOvertime(v_ptr) {
		delFlowFromPost(v_ptr)
	}

	// ADD flows at time (BGPUPDATE - syncdevi) to PriMaps
	for flag = Flow_queue.CsOnePriEndOvertime(v_ptr); flag; flag = Flow_queue.CsOnePriEndOvertime(v_ptr) {
		addFlow2Pri(v_ptr)
	}

	// Delete outdated(before delay+2*agetime+2*syncdevi) entry in PriRoute2Dst and PriDst2Route
	for flag = Flow_queue.CsPopPriStartOverTime(v_ptr); flag; flag = Flow_queue.CsPopPriStartOverTime(v_ptr) {
		delFlowFromPri(v_ptr)
	}

	// For each update BU at this time
	// 		Calculate Update Effect scope
	// If type is add: get dst_IP list that use BU after BGPUPDATE.
	//		For each IP, if its route queue head is quite BU, we think this IP is affected by BU
	//		Find this IP in
	for v, flag := Updata_queue.CsPopOverTime(utime - delay - agetime); flag; v, flag = Updata_queue.CsPopOverTime(utime - delay - agetime - syncdevi) {
		GivenUpdate(&v)
	}

}

var ipLoginfo bgp.IpLogInfo

func GivenUpdate(bu *bgp.BgpInfo) {
	SaveBgpUpdate(bu)
	// Add: find post ip_list according to Route
	if bu.Msg_type == bgp.BGP_ADD {
		rp := uint64(bu.New_ip_addr)>>(32-bu.New_ip_prefix)<<(40-bu.New_ip_prefix) + uint64(bu.New_ip_prefix)
		ipLoginfo.PostRoute = rp
		fmt.Printf("\033[33mUpdate ADD :\033[0m %+v\n", ipLoginfo)
		for k, v := range postRoute2Dst[rp] {
			ipLoginfo.DstIp = k
			ipLoginfo.PostFlow = v
			route, ok := priDst2Route[k]
			if ok {
				ipLoginfo.PriRoute = route[len(route)-1].RoutePrefix
				ipLoginfo.PriFlow = route[len(route)-1].Size
			} else {
				ipLoginfo.PriRoute = 0
				ipLoginfo.PriFlow = 0
			}
			SaveDetailInfo(ipLoginfo)
		}
	} else if bu.Msg_type == bgp.BGP_DELETE {
		rp := uint64(bu.Old_ip_addr)>>(32-bu.Old_ip_prefix)<<(40-bu.Old_ip_prefix) + uint64(bu.Old_ip_prefix)
		ipLoginfo.PriRoute = rp
		for k, v := range priRoute2Dst[rp] {
			ipLoginfo.DstIp = k
			ipLoginfo.PriFlow = v
			route, ok := postDst2Route[k]
			if ok {
				ipLoginfo.PostRoute = route[0].RoutePrefix
				ipLoginfo.PostFlow = route[0].Size
			} else {
				ipLoginfo.PostRoute = 0
				ipLoginfo.PostFlow = 0
			}
			SaveDetailInfo(ipLoginfo)
		}
	} else if bu.Msg_type == bgp.BGP_UPDATE {
		// check availability
	} else {
		util.PanicError(errors.New("func GivenUpdate: "), "Invalid Msg_type\n")
	}
}

func SaveBgpUpdate(bu *bgp.BgpInfo) {
	// fmt.Printf("BGP update: %#v\n", *bu)
	// if bu.Msg_type == bgp.BGP_ADD {
	// 	fmt.Printf("\033[31mGivenUpdate: Handling BGP ADD\033[0m\n")
	// } else if bu.Msg_type == bgp.BGP_DELETE {
	// 	fmt.Printf("\033[32mGivenUpdate: Handling BGP DELETE\033[0m\n")
	// } else if bu.Msg_type == bgp.BGP_UPDATE {
	// 	fmt.Printf("\033[34mGivenUpdate: Handling BGP UPDATE\033[0m\n")
	// }
}

func SaveDetailInfo(ipLoginfo bgp.IpLogInfo) {
	fmt.Printf("\033[33mDetailed : %+v\033[0m\n", ipLoginfo)
}
