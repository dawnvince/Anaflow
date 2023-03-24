package main

import (
	"anaflow/src/anaflow"
	"anaflow/src/bgp"
	"anaflow/src/util"
	"fmt"

	"github.com/spf13/viper"
)

func main() {
	// read config from config.toml
	viper.SetConfigFile("./config.toml")
	err := viper.ReadInConfig()
	util.PanicError(err, "Config Set error.")

	server_list := viper.GetStringSlice("url.servers")
	url_path := viper.GetString("url.base_path")
	interval := viper.GetInt("query_params.interval")
	limit := interval * viper.GetInt("query_params.limit_per_sec")

	update_queue := util.NewGCsqueue[bgp.BgpInfo]()
	go anaflow.RunBgpReceiver(update_queue)

	// flow_queue := util.NewGCsqueue[bgp.Flow]()
}
