package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	ws "github.com/gorilla/websocket"
	"github.com/panjf2000/ants/v2"
	"github.com/segmentio/ksuid"
	"golang.org/x/sync/errgroup"
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
	"sync/atomic"
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

var p, _ = ants.NewPool(10000)

func doSync() {
	//conn, err := EstablishConnection()
	//if err != nil {
	//	Log.Errorf("Connect to %v server error", GConf.Server)
	//	Log.Errorln(err.Error())
	//	return
	//}
	//defer func(c *ws.Conn) {
	//	_ = c.WriteControl(
	//		ws.CloseMessage,
	//		ws.FormatCloseMessage(ws.CloseNormalClosure, ""),
	//		time.Now().Add(time.Second*5),
	//	)
	//	_ = c.Close()
	//}(conn)

	//list, err := GetPendingFileList(conn)
	//if err != nil {
	//	Log.Errorln("GetPendingFileList ", err)
	//	return
	//}

	l, err := fs.CollectFileList(GConf.Dest)
	if err != nil {
		Log.Errorln("CollectFileList error", err)
		return
	}
	var list []PendingFileInfo
	for k, fp := range l {
		list = append(list, PendingFileInfo{
			Path:    k,
			Attr:    fp.Attr,
			Exist:   true,
			ModTime: fp.ModTime,
		})
	}

	var (
		fList []PendingFileInfo
	)

	for _, fp := range list {
		//Log.Debugln("sync file :", fp.Path, fp.Exist, fp.Attr)
		if fp.Attr != TFile {
			err := os.MkdirAll(path.Join(GConf.Dest, fp.Path), os.ModePerm)
			if err != nil {
				Log.Errorln("mkdir ", err)
				return
			}
			continue
		}
		fList = append(fList, fp)
	}
	group := errgroup.Group{}
	group.SetLimit(10)
	errFlag := atomic.Bool{}
	errFlag.Store(false)
	signatureList := make([][]*syn.BlockSignature, len(fList))

	for i := range fList {
		if errFlag.Load() {
			break
		}
		group.Go(func() error {
			f := fList[i]
			fp := path.Join(GConf.Dest, f.Path)
			_, err := os.Stat(fp)
			if err != nil {
				if os.IsNotExist(err) {
					Log.Debugln("File Not Exist")
					return nil
				}
				Log.Errorln("stat error", err)
				return err
			}

			file, err := os.Open(fp)
			if err != nil {
				Log.Errorln("Open File Error", err)
				return err
			}
			defer func(f *os.File) {
				if err := f.Close(); err != nil {
					Log.Warn("Close File Error")
				}
			}(file)
			signatures, err := syn.GetTotalSignatures(file, md5.New())
			if err != nil {
				Log.Errorln("GetTotalSignatures error", err)
				return err
			}
			signatureList[i] = signatures
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		Log.Errorln("SyncFile error", err)
	}

	Log.Debugln("Len ", len(fList), "Len2 ", len(signatureList))
	marshal, _ := json.Marshal(signatureList)
	//compress, err := util.ZstdCompress(marshal)
	//if err != nil {
	//	Log.Errorln("ZstdCompress Err")
	//}
	os.WriteFile("test.json", marshal, 0666)
	file, err := os.ReadFile("test.json")
	if err != nil {
		Log.Errorln("ReadFile")
		return
	}

	var ss [][]*syn.BlockSignature

	err = json.Unmarshal(file, &ss)
	if err != nil {
		Log.Errorln("Unmarshal")
	}
	Log.Debugln("Len3 ", len(ss))

	//if err := SyncFile(fp, conn); err != nil {
	//	Log.Errorln("SyncFile error", err)
	//	return
	//}
}

func SyncFile(f PendingFileInfo, c *ws.Conn) error {
	data, err := doSendRequest(f, c)
	if err != nil {
		return err
	}

	destPath := path.Join(GConf.Dest, f.Path)
	if f.Exist {

	}
	origin, err := os.OpenFile(destPath, os.O_RDONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		Log.Errorln("OpenFile error", err)
		return err
	}

	hash := h.GetHash()
	defer hash.Close()

	// SIG
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

	// DONE
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

	// SYN
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
