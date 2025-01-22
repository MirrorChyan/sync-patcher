package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	ws "github.com/gorilla/websocket"
	//h "mirrorc-sync/internel/hash"
	. "mirrorc-sync/internel/log"
	"mirrorc-sync/internel/pb"
	. "mirrorc-sync/internel/shared"
	"mirrorc-sync/internel/syn"
	"mirrorc-sync/internel/util"
	"net/http"
	"os"
	"path"
)

func GetParams(r *http.Request) *Params {
	return &Params{
		CDK: r.Header.Get(CDK),
		RID: r.Header.Get(RID),
		SID: r.Header.Get(SID),
	}
}

func Sync(w http.ResponseWriter, r *http.Request) {
	var (
		source string
		err    error
	)
	if source, err = Auth(r); err != nil {
		Log.Warnln("Auth: %v", err)
		return
	}
	u := &ws.Upgrader{
		HandshakeTimeout: 0,
		ReadBufferSize:   1024,
		WriteBufferSize:  2048,
		CheckOrigin:      func(r *http.Request) bool { return true },
	}
	conn, err := u.Upgrade(w, r, w.Header())
	if err != nil {
		Log.Errorln("Upgrade error: ", err)
		return
	}
	defer func(c *ws.Conn) {
		err := conn.Close()
		if err != nil {
			Log.Errorln("Server Conn Close error: ", err)
		}

	}(conn)

	if err := HandleConnection(r, conn, source); err != nil {
		Log.Errorln("HandleConnection %v", err)

	}
}

func Auth(r *http.Request) (s string, err error) {

	params := GetParams(r)

	s = "D:\\Project\\go\\mirrorc-sync\\tmp\\source"
	Log.Debugln(params)
	return
}

func HandleConnection(r *http.Request, conn *ws.Conn, source string) error {
	err := ProcessConnecting(r, conn, source)
	if err != nil {
		return err
	}

	return ProcessBinaryMessage(conn, source)
}

func ProcessConnecting(r *http.Request, m *ws.Conn, source string) error {

	Log.Debugln("Connected: ", r.RemoteAddr)

	files, err := GetResource(source)
	if err != nil {
		return err
	}

	info := &pb.Payload{
		Code: 0,
		Type: LST,
		Data: &pb.Payload_File{
			File: files,
		},
	}

	if err := util.WriteProtoMessage(m, info); err != nil {
		Log.Errorln("WriteBinary: %v", err)
		return err
	}

	return nil
}

func HandleRequest(c *ws.Conn, data *pb.Payload, source string) error {
	key := data.GetKey()
	Log.Debugln("HandleRequest path: ", key)

	Log.Debugln("Receive SIG")
	table, err := doReceiveSig(c)
	if err != nil {
		return err
	}

	Log.Debugln("Send SIG")
	if err := doSendSyn(c, source, key, table); err != nil {
		return err
	}

	Log.Debugln("Send FIN")
	if err := doSendFin(c); err != nil {
		return err
	}
	return nil
}

func doReceiveSig(c *ws.Conn) (map[uint32][]syn.BlockSignature, error) {
	return syn.LookUpTable(func() (*syn.BlockSignature, error) {
		message, err := util.ReadProtoMessage(c)
		if err != nil {
			return nil, err
		}
		switch message.GetType() {
		case DONE:
			return nil, nil
		case SIG:
			sig := message.GetSignature()
			return &syn.BlockSignature{
				Index:  sig.Index,
				Strong: sig.Strong,
				Weak:   sig.Weak,
			}, nil
		}
		return nil, errors.New("type mismatch")
	})
}

func doSendSyn(c *ws.Conn, s, key string, table map[uint32][]syn.BlockSignature) error {
	source, err := os.Open(path.Join(s, key))
	if err != nil {
		Log.Errorln("open file error:", err)
		return err
	}
	defer func(open *os.File) {
		err := open.Close()
		if err != nil {
			fmt.Println("close file error:", err)
		}
	}(source)

	//hash := h.GetHash()
	//hash.Reset()
	//defer hash.Close()

	return syn.Sync(source, md5.New(), table, func(op syn.BlockOperation) error {
		info := &pb.Payload{
			Code: 0,
			Type: SYN,
			Data: &pb.Payload_Operation{
				Operation: &pb.BlockOperation{
					Index: op.Index,
					Data:  op.Data,
				},
			},
		}
		return util.WriteProtoMessage(c, info)
	})
}

func doSendFin(c *ws.Conn) error {
	info := &pb.Payload{
		Code: 0,
		Type: FIN,
	}
	return util.WriteProtoMessage(c, info)
}

func ProcessBinaryMessage(c *ws.Conn, source string) error {
	for {
		data, err := util.ReadProtoMessage(c)
		if err != nil {
			var e *ws.CloseError
			if errors.As(err, &e) && e.Code == ws.CloseAbnormalClosure {
				return nil
			}
			return err
		}
		if data.GetType() != REQ {
			return errors.New("type mismatch")
		}
		err = HandleRequest(c, data, source)
		if err != nil {
			Log.Errorln("HandleRequest err", err)
		}
	}
}
