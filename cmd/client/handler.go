package main

import (
	"errors"
	ws "github.com/gorilla/websocket"
	"github.com/segmentio/ksuid"
	"io"
	"mirrorc-sync/internel/fs"
	h "mirrorc-sync/internel/hash"
	. "mirrorc-sync/internel/log"
	"mirrorc-sync/internel/pb"
	. "mirrorc-sync/internel/shared"
	"mirrorc-sync/internel/syn"
	"mirrorc-sync/internel/util"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

func EstablishConnection() (*ws.Conn, error) {

	re := regexp.MustCompile(`^(https?|wss?)://`)
	addr := re.ReplaceAllString(GConf.Server, "")
	Log.Debugln("server address:", addr)
	var suffix string
	if GConf.SSL {
		suffix = "wss://"
	} else {
		suffix = "ws://"
	}

	url := strings.Join([]string{suffix, addr, ApiPrefix}, "")
	Log.Debugln("url:", url)
	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func GetPendingFileList(c *ws.Conn) ([]PendingFileInfo, error) {
	payload, err := util.ReadProtoMessage(c)
	if err != nil {
		Log.Debugln("ReadProtoMessage err", err)
		return nil, err
	}
	if payload.Type != LST {
		Log.Debugln("type", GetTypeName(payload.Type))
		return nil, errors.New("type mismatch")
	}

	remote := payload.GetFile().Info
	var source = make(map[string]IFileInfo)

	for k, v := range remote {
		source[k] = IFileInfo{
			Attr:    v.Attr,
			ModTime: v.ModTime,
		}
	}

	dest, err := fs.CollectFileList(GConf.Dest)
	if err != nil {
		return nil, err
	}

	if GConf.Delete {
		removed := fs.GetBeRemovedFileInfo(source, dest)
		err = fs.ClearDestDir(GConf.Dest, removed)
		if err != nil {
			Log.Errorln("remove dest dir error")
			return nil, err
		}
	}

	return fs.GetPendingFileList(source, dest), nil
}

func doSync() {
	conn, err := EstablishConnection()
	if err != nil {
		Log.Errorf("Connect to %v server error", GConf.Server)
		Log.Errorln(err.Error())
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
		Log.Errorln("GetPendingFileList ", err)
		return
	}

	var (
		fList = list
	)

	for _, fp := range fList {
		Log.Debugln("sync file :", fp.Path, fp.Exist, fp.Attr)
		if fp.Attr != TFile {
			err := os.MkdirAll(path.Join(GConf.Dest, fp.Path), os.ModePerm)
			if err != nil {
				Log.Errorln("mkdir ", err)
				return
			}
			continue
		}

		if err := SyncFile(fp, conn); err != nil {
			Log.Errorln("SyncFile error", err)
			return
		}
	}
}

func SyncFile(f PendingFileInfo, c *ws.Conn) error {
	data, err := doSendRequest(f, c)
	if err != nil {
		return err
	}

	destPath := path.Join(GConf.Dest, f.Path)
	origin, err := os.OpenFile(destPath, os.O_RDONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		Log.Errorln("OpenFile error", err)
		return err
	}

	hash := h.GetHash()
	defer hash.Close()

	err = syn.Signatures(origin, hash, func(sig *syn.BlockSignature) error {
		data := &pb.Payload{
			Code: 0,
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
			Log.Errorln("WriteProtoMessage error", err)
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	data = &pb.Payload{
		Code: 0,
		Type: DONE,
		Key:  &f.Path,
	}
	if err := util.WriteProtoMessage(c, data); err != nil {
		Log.Errorln("WriteProtoMessage error", err)
		return err
	}

	id := ksuid.New().String()
	tmp := strings.Join([]string{f.Path, id}, ".")
	dest, err := os.OpenFile(path.Join(GConf.Dest, tmp), os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		return err
	}
	_, err = origin.Seek(0, io.SeekStart)
	if err != nil {
		Log.Errorln("origin.Seek error", err)
		return err
	}

	err = syn.Apply(dest, origin, func() (*syn.BlockOperation, error) {
		payload, err := util.ReadProtoMessage(c)
		if err != nil {
			Log.Errorln("GetPayload error", err)
			return nil, err
		}
		switch payload.GetType() {
		case FIN:
			Log.Debugln("Receive FIN")
			return nil, nil
		case SYN:
			op := payload.GetOperation()
			Log.Debugf("Receive SYN size %v kb", float32(len(op.Data))/1024.0)
			return &syn.BlockOperation{
				Index: op.GetIndex(),
				Data:  op.GetData(),
			}, nil
		}
		return nil, nil
	})

	if err := origin.Close(); err != nil {
		Log.Errorln("os.Close Err", err)
		return err
	}

	if err != nil {
		Log.Errorln("Apply error", err)
		_ = dest.Close()
		_ = os.Remove(path.Join(GConf.Dest, tmp))
		return err
	}

	err = dest.Close()
	if err != nil {
		Log.Errorln("dest.Close error", err)
		return err
	}
	err = os.Rename(path.Join(GConf.Dest, tmp), destPath)

	if err != nil {
		Log.Errorln("Rename error", err)
		return err
	}
	t := time.Unix(f.ModTime, 0)
	err = os.Chtimes(destPath, time.Time{}, t)
	if err != nil {
		Log.Errorln("Chtimes error", err)
		return err
	}

	return nil
}

func doSendRequest(f PendingFileInfo, c *ws.Conn) (*pb.Payload, error) {
	data := &pb.Payload{
		Code: 0,
		Type: REQ,
		Key:  &f.Path,
	}
	err := util.WriteProtoMessage(c, data)
	if err != nil {
		return nil, err
	}
	Log.Debugln("Send REQ")
	return data, err
}
