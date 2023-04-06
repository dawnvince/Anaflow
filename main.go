package main

import (
	"anaflow/src/anaflow"
	"anaflow/src/util"
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/viper"
)

func main() {
	// read config from config.toml
	viper.SetConfigFile("./config.toml")
	err := viper.ReadInConfig()
	util.PanicError(err, "Config Set error.")

	server_list := viper.GetStringSlice("url.servers")
	url_path := viper.GetString("url.base_path")
	interval := viper.GetInt64("query_params.interval")
	loki_delay := viper.GetInt64("query_params.delay")
	limit := interval * viper.GetInt64("query_params.limit_per_sec")

	delay := viper.GetInt64("time_settings.delay")
	agetime := viper.GetInt64("time_settings.agetime")
	syncdevi := viper.GetInt64("time_settings.syncdevi")

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGTERM, syscall.SIGINT)

	ticker_flow := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker_flow.Stop()
	ticker_update := time.NewTicker(1 * time.Second)
	defer ticker_update.Stop()

	log_file := "./scope.log"
	file, err := os.OpenFile(log_file, os.O_CREATE | os.O_APPEND | os.O_WRONLY, 0666)
	util.CheckError(err)
	defer file.Close()

	anaflow.File_writer = bufio.NewWriter(file)

	go anaflow.RunBgpReceiver(anaflow.Updata_queue)

	go func() {
		for {
			ut := <-ticker_update.C
			anaflow.GivenCurrentTime(int64(ut.Unix()), delay, agetime, syncdevi)
		}
	}()

	for {
		select {
		case t := <-ticker_flow.C:
			utime := t.Unix() - loki_delay
			for _, u := range server_list {
				url := fmt.Sprintf("%s%s&start=%d000000000&end=%d999999999&limit=%d", u, url_path, utime-interval, utime-1, limit)

				go anaflow.RequestLoki(utime, url)
			}
		case s := <-sigint:
			ticker_flow.Stop()
			ticker_update.Stop()
			fmt.Println("Receive Signal s=", s)
			os.Exit(1)
		}
	}
}
