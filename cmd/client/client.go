package main

import (
	"crypto/md5"
	"errors"
	ws "github.com/gorilla/websocket"
	"github.com/segmentio/ksuid"
	"io"
	"mirrorc-sync/internel/fs"
	"mirrorc-sync/internel/log"
	"mirrorc-sync/internel/pb"
	. "mirrorc-sync/internel/shared"
	"mirrorc-sync/internel/syn"
	"mirrorc-sync/internel/util"
	"os"
	"path"
	"strings"
	"time"
)

var (
	Dest = "dest"
	Addr = "localhost:5000"
)

func EstablishConnection() (*ws.Conn, error) {
	url := strings.Join([]string{"ws://", Addr, "/ws"}, "")
	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func GetPendingFileList(c *ws.Conn) ([]PendingFileInfo, error) {
	payload, err := util.ReadProtoMessage(c)
	if err != nil {
		return nil, err
	}
	if payload.GetType() != LST {
		return nil, errors.New("type mismatch")
	}

	source := payload.GetFile().Info

	dest, err := fs.CollectFileList(Dest)
	if err != nil {
		return nil, err
	}

	removed := fs.GetBeRemovedFileInfo(source, dest)
	err = fs.ClearDestDir(Dest, removed)
	if err != nil {
		log.Errorln("remove dest dir error")
		return nil, err
	}

	list := fs.GetPendingFileList(source, dest)

	return list, nil
}

func doSync() {
	conn, err := EstablishConnection()
	if err != nil {
		log.Errorln("Dial: %v", err)
		return
	}
	defer func(c *ws.Conn) {
		_ = c.WriteControl(
			ws.CloseMessage,
			ws.FormatCloseMessage(ws.CloseNormalClosure, ""),
			time.Now().Add(time.Second*5),
		)
		_ = c.Close()
	}(conn)

	list, err := GetPendingFileList(conn)
	if err != nil {
		var e *ws.CloseError
		if errors.As(err, &e) && e.Code == ws.CloseAbnormalClosure {
			return
		}
		log.Errorln("GetPendingFileList ", err)
		return
	}

	var (
		fList = list
	)

	for _, fp := range fList {
		log.Errorln("sync file :", fp.Path, fp.Exist, fp.Attr)
		if fp.Attr != TFile {
			err := os.MkdirAll(path.Join(Dest, fp.Path), os.ModePerm)
			if err != nil {
				log.Errorln("MkdirAll ", err)
				return
			}
			continue
		}
		if err := SyncFile(fp, conn); err != nil {
			log.Errorln("SyncFile error", err)
			return
		}
	}
}

func SyncFile(f PendingFileInfo, c *ws.Conn) error {
	data, err := doSendRequest(f, c)
	if err != nil {
		return err
	}

	origin, err := os.OpenFile(path.Join(Dest, f.Path), os.O_RDONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		log.Errorln("OpenFile error", err)
		return err
	}

	err = syn.Signatures(origin, md5.New(), func(sig *syn.BlockSignature) error {
		data := &pb.Payload{
			Code: 0,
			Msg:  "sig",
			Type: SIG,
			Key:  &f.Path,
			Data: &pb.Payload_Signature{
				Signature: &pb.BlockSignature{
					Index:  sig.Index,
					Strong: sig.Strong,
					Weak:   sig.Weak,
				},
			},
		}
		if err := util.WriteProtoMessage(c, data); err != nil {
			log.Errorln("WriteProtoMessage error", err)
			return err
		}
		log.Errorln("Send SIG")
		return nil
	})
	if err != nil {
		return err
	}

	data = &pb.Payload{
		Code: 0,
		Msg:  "done",
		Type: DONE,
		Key:  &f.Path,
	}
	if err := util.WriteProtoMessage(c, data); err != nil {
		log.Errorln("WriteProtoMessage error", err)
		return err
	}
	log.Errorln("Send DONE")

	id := ksuid.New().String()
	tmp := strings.Join([]string{f.Path, id}, ".")
	dest, err := os.OpenFile(path.Join(Dest, tmp), os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		return err
	}
	_, err = origin.Seek(0, io.SeekStart)
	if err != nil {
		log.Errorln("origin.Seek error", err)
		return err
	}

	err = syn.Apply(dest, origin, func() (*syn.BlockOperation, error) {
		payload, err := util.ReadProtoMessage(c)
		if err != nil {
			log.Errorln("GetPayload error", err)
			return nil, err
		}
		switch payload.GetType() {
		case FIN:
			log.Debugln("Receive FIN")
			return nil, nil
		case SYN:
			op := payload.GetOperation()
			log.Debugln("Receive SYN SIZE", len(op.Data)/1024.0)
			return &syn.BlockOperation{
				Index: op.GetIndex(),
				Data:  op.GetData(),
			}, nil
		}
		return nil, nil
	})

	if err := origin.Close(); err != nil {
		log.Errorln("os.Close Err", err)
		return err
	}

	if err != nil {
		log.Errorln("Apply error", err)
		_ = dest.Close()
		_ = os.Remove(path.Join(Dest, tmp))
		return err
	}

	err = dest.Close()
	if err != nil {
		log.Errorln("dest.Close error", err)
		return err
	}
	err = os.Rename(path.Join(Dest, tmp), path.Join(Dest, f.Path))
	if err != nil {
		log.Errorln("Rename error", err)
		return err
	}

	return nil
}

func doSendRequest(f PendingFileInfo, c *ws.Conn) (*pb.Payload, error) {
	data := &pb.Payload{
		Code: 0,
		Msg:  "init",
		Type: REQ,
		Key:  &f.Path,
	}
	err := util.WriteProtoMessage(c, data)
	if err != nil {
		return nil, err
	}
	log.Debugln("Send REQ")
	return data, err
}

func main() {
	log.InitLogger()
	doSync()
}
