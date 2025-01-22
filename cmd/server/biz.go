package main

import (
	"mirrorc-sync/internel/fs"
	"mirrorc-sync/internel/pb"
)

func GetResource(source string) (*pb.FileDescriptionInfo, error) {

	list, err := fs.CollectFileList(source)
	if err != nil {
		return nil, err
	}

	var info = make(map[string]*pb.FileDescriptionInfo_Info)
	for k, v := range list {
		info[k] = &pb.FileDescriptionInfo_Info{
			Attr:    v.Attr,
			ModTime: v.ModTime,
		}
	}

	return &pb.FileDescriptionInfo{
		Info: info,
	}, nil
}
