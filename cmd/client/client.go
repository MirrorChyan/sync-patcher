package main

import (
	"log"
	h "mirrorc-sync/internel/hash"
	. "mirrorc-sync/internel/log"
	"net/http"
	_ "net/http/pprof"
	"time"
)

// todo: remove this
func debug() {

}

func main() {
	go func() {
		log.Println(http.ListenAndServe(":6061", nil))
	}()
	ParseConfig()
	h.StartHash()
	InitLogger()
	debug()
	ts := time.Now()
	doSync()
	Log.Infof("Sync End %v ms", time.Now().Sub(ts).Milliseconds())
}
