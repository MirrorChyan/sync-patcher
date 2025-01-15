package util

import (
	"errors"
	ws "github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"log"
	"mirrorc-sync/internel/pb"
)

func WriteProtoMessage(c *ws.Conn, data *pb.Payload) error {
	buf, err := proto.Marshal(data)
	if err != nil {
		log.Println("proto Marshal err", err)
		return err
	}
	return c.WriteMessage(ws.BinaryMessage, buf)
}

func ReadProtoMessage(c *ws.Conn) (*pb.Payload, error) {
	mt, buf, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}
	if mt != ws.BinaryMessage {
		return nil, errors.New("not binary message")
	}
	p := &pb.Payload{}
	if err := proto.Unmarshal(buf, p); err != nil {
		log.Printf("Unmarshal: %v", err)
		return nil, err
	}
	return p, nil
}
