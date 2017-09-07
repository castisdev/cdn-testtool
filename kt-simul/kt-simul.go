package main

import (
	"log"
	"path"
	"time"

	"github.com/castisdev/cdn-testtool/kt-simul/ktsimul"
	"github.com/castisdev/cilog"
	_ "github.com/go-sql-driver/mysql"
)

func setLog(dir, module, moduleVersion string, minLevel cilog.Level) {
	cilog.Set(cilog.NewLogWriter(dir, module, 10*1024*1024), module, moduleVersion, minLevel)
}

func main() {
	cfg, err := ktsimul.NewConfig("kt-simul.yml")
	if err != nil {
		log.Fatal(err)
	}

	setLog(cfg.LogDir, "kt-simul", "1.0.0", cfg.LogLevel)
	cilog.Infof("program started")

	copyFiles(cfg)

	stat := ktsimul.NewProcessingStat()
	go stat.Start()

	for _, v := range cfg.Locals {
		go processSetupForLocal(cfg, v, stat)
	}

	processDelivery(cfg, stat)
}

func copyFiles(cfg *ktsimul.Config) {
	aclient := "adsadapter-client"
	target := path.Join(cfg.RemoteADSAdapterClientDir, aclient)
	copy, err := ktsimul.RemoteCopyIfNotExists(cfg, aclient, cfg.EADSIP, target)
	if err != nil {
		log.Fatalf("failed to remote-copy, %v", err)
	}
	if copy {
		cilog.Infof("remote copied %v to %v", aclient, cfg.EADSIP)
	}

	for _, file := range cfg.SourceFiles {
		target := path.Join(cfg.RemoteOriginFileDir, file)
		copy, err := ktsimul.RemoteCopyIfNotExists(cfg, file, cfg.EADSIP, target)
		if err != nil {
			log.Fatalf("failed to remote-copy, %v", err)
		}
		if copy {
			cilog.Infof("remote copied %v to %v", file, cfg.EADSIP)
		}
	}

	for _, bin := range cfg.VODClientBins {
		for _, ip := range cfg.VODClientIPs {
			target := path.Join(cfg.RemoteVodClientDir, bin)
			copy, err := ktsimul.RemoteCopyIfNotExists(cfg, bin, ip, target)
			if err != nil {
				log.Fatalf("failed to remote-copy, %v", err)
			}
			if copy {
				cilog.Infof("remote copied %v to %v", bin, ip)
			}
		}
	}
}

func processDelivery(cfg *ktsimul.Config, stat *ktsimul.ProcessingStat) {
	for {
		<-time.After(cfg.DeliverSleep)
		err := ktsimul.RunDeliverOne(cfg, stat)
		if err != nil {
			cilog.Errorf("failed to deliver, %v", err)
		}
	}
}

func processSetupForLocal(cfg *ktsimul.Config, localCfg ktsimul.LocalConfig, stat *ktsimul.ProcessingStat) {
	for {
		<-time.After(localCfg.SetupPeriod)
		go func() {
			err := ktsimul.RunSetupOne(cfg, localCfg)
			if err != nil {
				cilog.Errorf("failed to setup, %v", err)
			}
		}()
	}
}
