package main

import (
	"mirrorc-sync/internel/fs"
	"mirrorc-sync/internel/pb"
	"mirrorc-sync/internel/shared"
)

func GetResource(params *shared.Params) (*pb.FileDescriptionInfo, error) {

	list, err := fs.CollectFileList(SOURCE)
	if err != nil {
		return nil, err
	}

	return &pb.FileDescriptionInfo{
		Info: list,
	}, nil
}
