package main

import (
	"log"
	h "mirrorc-sync/internel/hash"
	. "mirrorc-sync/internel/log"
	"mirrorc-sync/internel/shared"
	"net/http"
	_ "net/http/pprof"
	"time"
)

const (
	ServerAddr = ":5000"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe(":6060", nil))
	}()
	InitLogger()
	h.StartHash()
	defer h.EndHash()

	Log.Infoln("Start Server")

	http.HandleFunc(shared.ApiPrefix, func(w http.ResponseWriter, r *http.Request) {
		Log.Infoln("Sync Start")
		ts := time.Now()
		Sync(w, r)
		Log.Infof("Sync End %v ms", time.Now().Sub(ts).Milliseconds())
	})
	if err := http.ListenAndServe(ServerAddr, nil); err != nil {
		Log.Errorln("ListenAndServe: %v", err)
	}

}
