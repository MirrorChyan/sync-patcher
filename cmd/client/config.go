package main

import (
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/shirou/gopsutil/cpu"
	"os"
)

type Config struct {
	CDK    string `short:"c" long:"cdk" description:"MirrorChyan cdkey"`
	RID    string `short:"r" long:"rid" description:"MirrorChyan resource id"`
	SID    string `short:"s" long:"sid" description:"optional hardware id"`
	Dest   string `short:"t" long:"target" description:"optional target directory to be synchronized If it does not exist, it is the $CWD/dest"`
	Server string `long:"server" description:"optional specify server address"`
	SSL    bool   `long:"ssl" description:"optional specify whether to use tls"`
	Delete bool   `long:"delete" description:"optional specify whether it is consistent with the remote side"`
}

var GConf *Config

func ParseConfig() {
	info, err := cpu.Info()
	if err != nil {
		fmt.Println("get PhysicalID info error: ", err.Error())
		return
	}

	GConf = &Config{
		SID:    info[0].PhysicalID,
		Server: "",
		Dest:   "./dest",
		SSL:    true,
	}

	_, err = flags.Parse(GConf)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	if GConf.CDK == "" || GConf.RID == "" {
		fmt.Println("cdk or resource id can not be empty, please use -h to see help")
		//os.Exit(1)
	}
}
