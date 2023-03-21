package main

import (
	"anaflow/src/anaflow"
	"anaflow/src/bgp"
	"anaflow/src/util"
)

func main() {


	update_queue := util.NewGCsqueue[bgp.BgpInfo]()
	
	go anaflow.RunBgpReceiver(update_queue)

	// flow_queue := util.NewGCsqueue[bgp.Flow]()
}