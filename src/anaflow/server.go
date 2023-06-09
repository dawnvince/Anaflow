/*
Two types of servers: BGP update receiver and Flow receiver

BGP update receiver(BUR) is an almost real-time info receiver in a passive way. When the peer sends an update, BUR receive the update and add it to BgpUpdateQueue(over 10 messages/s). The BIRD and BUR communicate in ByteStream way.

Flow receiver(FR) polls flow information from flow collection system at intervals. When a massive of flows arrive(over 100,000 streams every 5 min), FR adds them to FlowQueue. FR asks for flows by Loki API and receives them in JSON structure
*/
package anaflow

import (
	"anaflow/src/bgp"
	"anaflow/src/util"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/buger/jsonparser"
)

// BUR Implement
const buf_len = 1000

/* Packet Format
 		msg_type 		4 byte
		1: add RTE
		2: delete RTE
		3: change RTE attr

	OLD_RTE_INFO(all 0 if not exists)
		old_ip_addr 	4 byte
		old_ip_prefix 	4 byte
		old_nexthop 	4 byte
		old_first_asn 	4 byte
		old_path_len 	4 byte
		old_pref 		4 byte

	NEW_RTE_INFO(all 0 if not exists)
		new_ip_addr 	4 byte
		new_ip_prefix 	4 byte
		new_nexthop 	4 byte
		new_first_asn 	4 byte
		new_path_len 	4 byte
		new_pref 		4 byte
*/

func Packet2info(buf []byte, bgpinfo *bgp.BgpInfo) {
	byteBuffer := bytes.NewReader(buf)
	if err := binary.Read(byteBuffer, binary.LittleEndian, bgpinfo); err != nil {
		util.CheckError(err)
	}
}

func RunBgpReceiver(uq *util.GCsqueue[bgp.BgpInfo]) {
	socket_file := "/tmp/c2gsocket"
	socket_name := "unixgram"
	addr, err := net.ResolveUnixAddr(socket_name, socket_file)
	util.CheckError(err)
	syscall.Unlink(socket_file)

	listener, err := net.ListenUnixgram(socket_name, addr)
	util.CheckError((err))
	defer listener.Close()

	buf := make([]byte, buf_len)
	bgpinfo := new(bgp.BgpInfo)
	for {
		size, _, err := listener.ReadFromUnix(buf)
		util.CheckError(err)
		content := buf[:size]
		Packet2info(content, bgpinfo)
		// fmt.Printf("test result : %#v\n", bgpinfo)
		uq.CsPush(*bgpinfo, bgpinfo.Btime)
	}
}

// FR Implement

func RequestLoki(utime int64, url string) {
	resp, err := http.Get(url)
	util.PanicError(err, "Request Loki error")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	util.PanicError(err, "Read Loki Packet error")

	data := *dataPreprocess(body)
	Json2Flow(data)

	fmt.Printf("After RequestLoki, FlowQueue's length is %d\n", Flow_queue.GetLength())
}

/*
 * Parse JSON to struct Flow and Add it to FlowQueue
 * Using a fast JSON parser @github.com/buger/jsonparser
 * As original Log infomation encoding value as string format, func dataPreprocess is used to convert the string to valid JSON format
 */

// parse path needed for jsonparser
// share_path is used to locate the metadata
var shared_path []string = []string{"data", "result", "[0]", "values"}

// paths is used for detailed parsing
var paths = [][]string{
	{"[1]", "network", "bytes"},
	{"[1]", "source", "ip"},
	{"[1]", "destination", "ip"},
	{"[1]", "dstIP"},
	{"[1]", "dstPrefixLength"},
	{"[1]", "bgpSrcAsNumber"},
	{"[1]", "bgpDstAsNumber"},
	{"[1]", "observer", "ip"},
	{"[1]", "bgpNextHopAddress"},
	{"[1]", "event", "start"},
	{"[1]", "event", "end"},
	{"[1]", "netflow", "egress_interface"},
}

func dataPreprocess(bytes []byte) *[]byte {
	newbyte := make([]byte, 0, len(bytes))
	l := len(bytes)
	for i := 0; i < l; i++ {
		if bytes[i] == '"' && i < l-1 && bytes[i+1] == '{' {
			continue
		}
		if i > 0 && bytes[i] == '"' && bytes[i-1] == '}' {
			continue
		}
		if bytes[i] != '\\' {
			newbyte = append(newbyte, bytes[i])
		}
	}
	return &newbyte
}

func Json2Flow(data []byte) {
	jsonparser.ArrayEach(data, ParseEachElement, shared_path...)
}

func ParseEachElement(value []byte, dataType jsonparser.ValueType, offset int, err error) {
	var flow_entry bgp.Flow
	var tv int64
	jsonparser.EachKey(value,
		func(idx int, value []byte, vt jsonparser.ValueType, err error) {
			switch idx {
			case 0:
				tv, _ = jsonparser.ParseInt(value)
				flow_entry.Size = uint64(tv)
			case 1:
				flow_entry.Src_ip = util.IPbyte2int(value)
			case 2:
				flow_entry.Dst_ip = util.IPbyte2int(value)
			case 3:
				flow_entry.Route = util.IPbyte2int(value)
			case 4:
				tv, _ = jsonparser.ParseInt(value)
				flow_entry.Prefix = uint16(tv)
			case 5:
				tv, _ = jsonparser.ParseInt(value)
				flow_entry.Src_as = uint32(tv)
			case 6:
				tv, _ = jsonparser.ParseInt(value)
				flow_entry.Dst_as = uint32(tv)
			case 7:
				flow_entry.Observer_ip = util.IPbyte2int(value)
			case 8:
				flow_entry.Nh_ip = util.IPbyte2int(value)
			case 9:
				t, _ := time.Parse(time.RFC3339, string(value))
				flow_entry.Start_t = int64(t.Unix())
			case 10:
				t, _ := time.Parse(time.RFC3339, string(value))
				flow_entry.End_t = int64(t.Unix())
			case 11:
				tv, _ = jsonparser.ParseInt(value)
				flow_entry.Egress_id = uint16(tv)
			}
		}, paths...)

	AddFlow2Q(flow_entry)
}
