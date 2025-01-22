package main

import (
	"log"
	h "mirrorc-sync/internel/hash"
	. "mirrorc-sync/internel/log"
	"net/http"
	_ "net/http/pprof"
)

// todo: remove this
func debug() {
	GConf.Server = "ws://127.0.0.1:5000"
	GConf.Dest = "D:\\Project\\go\\mirrorc-sync\\tmp\\dest"
	GConf.SSL = false
}

func main() {
	go func() {
		log.Println(http.ListenAndServe(":6061", nil))
	}()
	ParseConfig()
	h.StartHash()
	InitLogger()
	debug()
	doSync()
}
