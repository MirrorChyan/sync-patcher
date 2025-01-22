package util

import (
	"errors"
	ws "github.com/gorilla/websocket"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	. "mirrorc-sync/internel/log"
	"mirrorc-sync/internel/pb"
	"mirrorc-sync/internel/shared"
)

func WriteProtoMessage(c *ws.Conn, data *pb.Payload) error {

	if err := processSendPayload(data); err != nil {
		return err
	}

	buf, err := proto.Marshal(data)
	if Log.Level().Enabled(zap.DebugLevel) {
		Log.Debugln("Send ", shared.GetTypeName(data.GetType()))
	}
	if err != nil {
		Log.Debugln("proto Marshal err", err)
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

	data := &pb.Payload{}
	if err := proto.Unmarshal(buf, data); err != nil {
		Log.Debugln("Unmarshal: ", err)
		return nil, err
	}

	if err := processReceivePayload(data); err != nil {
		return nil, err
	}

	if Log.Level().Enabled(zap.DebugLevel) {
		Log.Debugln("Receive ", shared.GetTypeName(data.GetType()))
	}

	return data, nil
}
