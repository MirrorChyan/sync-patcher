package util

import (
	"bytes"
	"github.com/klauspost/compress/zstd"
	. "mirrorc-sync/internel/log"
	"mirrorc-sync/internel/pb"
	"mirrorc-sync/internel/shared"
	"sync"
)

var (
	encoderPool = sync.Pool{
		New: func() interface{} {
			writer, err := zstd.NewWriter(nil)
			if err != nil {
				Log.Errorln("Zstd NewWriter", err)
			}
			return writer
		},
	}
	decoderPool = sync.Pool{
		New: func() interface{} {
			reader, err := zstd.NewReader(nil)
			if err != nil {
				Log.Errorln("Zstd NewReader", err)
			}
			return reader
		},
	}
)

func processSendPayload(data *pb.Payload) error {
	if data.Type == shared.SYN && len(data.GetOperation().Data) > 0 {
		op := data.GetOperation()
		payload, err := ZstdCompress(op.Data)
		if err != nil {
			Log.Errorln("ZstdCompress", err)
			return err
		}
		op.Data = payload
	}
	return nil
}

func processReceivePayload(data *pb.Payload) error {
	if data.Type == shared.SYN && len(data.GetOperation().Data) > 0 {
		op := data.GetOperation()
		payload, err := ZstdDecompress(op.Data)
		if err != nil {
			Log.Errorln("ZstdCompress", err)
			return err
		}
		op.Data = payload
	}
	return nil
}

func ZstdCompress(buf []byte) ([]byte, error) {

	buffer := bytes.NewBuffer(nil)
	writer := encoderPool.Get().(*zstd.Encoder)
	defer encoderPool.Put(writer)
	writer.Reset(buffer)

	_, err := writer.Write(buf)
	if err != nil {
		Log.Debugln("Zstd Write err", err)
		return nil, err
	}

	if err := writer.Close(); err != nil {
		Log.Debugln("Zstd Close err", err)
		return nil, err
	}
	return buffer.Bytes(), nil
}

func ZstdDecompress(buf []byte) ([]byte, error) {
	decoder := decoderPool.Get().(*zstd.Decoder)
	defer decoderPool.Put(decoder)

	err := decoder.Reset(bytes.NewBuffer(buf))
	if err != nil {
		Log.Debugln("Zstd Reset err", err)
		return nil, err
	}

	buffer := bytes.NewBuffer(nil)
	_, err = buffer.ReadFrom(decoder)
	if err != nil {
		Log.Debugln("Zstd ReadFrom err", err)
		return nil, err
	}
	return buffer.Bytes(), nil
}
