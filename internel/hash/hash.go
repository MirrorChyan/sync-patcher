package hash

import md5simd "github.com/minio/md5-simd"

import . "mirrorc-sync/internel/log"

var hs md5simd.Server

func GetHash() md5simd.Hasher {
	return hs.NewHash()
}

func StartHash() {
	hs = md5simd.NewServer()
}

func EndHash() {
	Log.Debugln("EndHash")
	hs.Close()
}
