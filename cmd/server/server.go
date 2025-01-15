package main

import (
	ws "github.com/gorilla/websocket"
	"mirrorc-sync/internel/log"
	"net/http"
	"time"
)

var SOURCE = "source"

func main() {
	log.InitLogger()
	http.HandleFunc("/ws", Sync)

	log.Infoln("Start Server")
	err := http.ListenAndServe(":5000", nil)

	if err != nil {
		log.Errorln("ListenAndServe: %v", err)
	}
}

func Sync(w http.ResponseWriter, r *http.Request) {
	if err := Auth(r); err != nil {
		log.Warnln("Auth: %v", err)
		return
	}
	var upgrader = &ws.Upgrader{
		HandshakeTimeout: 0,
		ReadBufferSize:   1024,
		WriteBufferSize:  2048,
		CheckOrigin:      func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, w.Header())
	defer func(c *ws.Conn) {
		err = c.WriteControl(
			ws.CloseMessage,
			ws.FormatCloseMessage(ws.CloseNormalClosure, ""),
			time.Now().Add(time.Second*5),
		)
		if err != nil {
			log.Errorln("WriteControl error", err)
		}
		err := conn.Close()
		if err != nil {
			log.Errorln("Close error: ", err)
		}

	}(conn)
	if err != nil {
		log.Errorln("Upgrade error: ", err)
		return
	}

	if err := HandleConnection(r, conn); err != nil {
		log.Errorln("HandleConnection %v", err)

	}
}
