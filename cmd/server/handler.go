package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	ws "github.com/gorilla/websocket"
	"github.com/segmentio/ksuid"
	"mirrorc-sync/internel/log"
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

func Auth(r *http.Request) error {
	params := GetParams(r)
	log.Debugln("%v\n", params)
	return nil
}

func HandleConnection(r *http.Request, conn *ws.Conn) error {
	session, err := ProcessConnecting(r, conn)
	if err != nil {
		return err
	}

	return ProcessBinaryMessage(conn, session)
}

func ProcessConnecting(r *http.Request, m *ws.Conn) (*ServerSession, error) {
	session := &ServerSession{
		Id:    ksuid.New().String(),
		Param: GetParams(r),
		List:  nil,
	}

	log.Errorln("Connected: %s\n", session.Id)

	files, err := GetResource(session.Param)
	if err != nil {
		return nil, err
	}

	info := &pb.Payload{
		Code: 0,
		Msg:  "LST file list",
		Type: LST,
		Data: &pb.Payload_File{
			File: files,
		},
	}

	if err := util.WriteProtoMessage(m, info); err != nil {
		log.Errorln("WriteBinary: %v", err)
		return nil, err
	}

	return session, nil
}

func HandleRequest(c *ws.Conn, data *pb.Payload) error {
	p := data.GetKey()
	log.Errorln("HandleRequest path: %v\n", p)

	log.Debugln("Receive SIG")
	table, err := doReceiveSig(c)
	if err != nil {
		return err
	}

	log.Debugln("Send SIG")
	if err := doSendSyn(c, err, p, table); err != nil {
		return err
	}

	log.Debugln("Send FIN")
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

func doSendSyn(c *ws.Conn, err error, p string, table map[uint32][]syn.BlockSignature) error {
	source, err := os.Open(path.Join(SOURCE, p))
	if err != nil {
		log.Errorln("open file error:", err)
		return err
	}
	defer func(open *os.File) {
		err := open.Close()
		if err != nil {
			fmt.Println("close file error:", err)
		}
	}(source)

	return syn.Sync(source, md5.New(), table, func(op *syn.BlockOperation) error {
		info := &pb.Payload{
			Code: 0,
			Msg:  "sync file",
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
		Msg:  "fin",
		Type: FIN,
	}
	return util.WriteProtoMessage(c, info)
}

func ProcessBinaryMessage(c *ws.Conn, msg *ServerSession) error {
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
		err = HandleRequest(c, data)
		if err != nil {
			log.Errorln("HandleRequest err", err)
		}
	}
}
