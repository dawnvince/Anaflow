package bgp

// BGP info type
const (
	BGP_ADD = iota + 1
	BGP_DELETE
	BGP_UPDATE
)

// Communication Message with BIRD
type BgpInfo struct {
	Msg_type int32
	Padding  int32 // used to handle alignment difference between C struct and GO struct

	Old_ip_addr   uint32
	Old_ip_prefix int32
	Old_nexthop   uint32
	Old_first_asn int32
	Old_path_len  int32
	Old_pref      int32

	New_ip_addr   uint32
	New_ip_prefix int32
	New_nexthop   uint32
	New_first_asn int32
	New_path_len  int32
	New_pref      int32

	Btime int64
}

/*
The corresponding relationships between struct Flow and JSON are listed as follows:
Size         source.bytes
Src_ip       source.ip
Dst_ip       destination.ip
Route_prefix netflow.destination_ipv4_address >> netflow_destination_ipv4_prefix_length
Src_as       netflow.bgp_source_as_number
Dst_as       netflow.bgp_destination_as_number
Observer_ip  observer.ip
Nh_ip        netflow.bgp_next_hop_ipv4_address// Nexthop ip
Start_t      netflow.flow_start_sys_up_time / 1000
End_t        netflow.flow_end_sys_up_time / 1000
Egress_id    netflow.egress_interface
*/

// {job="netflow"} != "ipv6" | json Size="source.bytes", Src_ip="source.ip", Dst_ip="destination.ip", Route="dstIP", Prefix="dstPrefixLength", Src_as="bgpSrcAsNumber", Dst_as="bgpDstAsNumber", Observer_ip="observer.ip", Nh_ip="bgpNextHopAddress", Start_t="flowStartSysUpTime", End_t="flowEndSysUpTime", Egress_id="egressInterface"

type Flow struct {
	Egress_id   uint16
	Prefix      uint16
	Route       uint32
	Src_ip      uint32
	Dst_ip      uint32
	Src_as      uint32
	Dst_as      uint32
	Observer_ip uint32 // Router that reports the flow
	Nh_ip       uint32 // Nexthop ip
	Start_t     int64
	End_t       int64
	Size        uint64
}

type IpInfo struct {
	RoutePrefix uint64 // uint32 IP + uint8 Prefix
	Size        uint64
}

type IpLogInfo struct {
	DstIp     uint32
	PriRoute  uint64
	PriFlow   uint64
	PostRoute uint64
	PostFlow  uint64
}
