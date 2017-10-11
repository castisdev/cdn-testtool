package main

import (
	"flag"
	"fmt"
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
	var isTest bool
	flag.BoolVar(&isTest, "test", false, "run test mode")
	flag.Parse()

	cfg, err := ktsimul.NewConfig("kt-simul.yml")
	if err != nil {
		log.Fatal(err)
	}

	if isTest {
		cilog.SetMinLevel(cilog.INFO)
		fmt.Print("check ip in config ...\n\n")
		result, okIPs := ktsimul.TestCfgIPs(cfg)
		fmt.Println(result)

		fmt.Print("check LSM in config ...\n\n")
		result = ktsimul.TestLSMs(cfg, okIPs)
		fmt.Println(result)
		return
	}

	setLog(cfg.LogDir, "kt-simul", "1.0.0", cfg.LogLevel)
	cilog.Infof("program started")

	copyFiles(cfg)

	stat := ktsimul.NewProcessingStat()
	go stat.Start()

	for _, v := range cfg.Locals {
		go processSetupForLocal(cfg, v, stat)
	}

	go processHBDelivery(cfg, stat)
	go processDeleteFile(cfg, stat)
	processDelivery(cfg, stat)
}

func copyFiles(cfg *ktsimul.Config) {
	adsIP := cfg.FileDeliver.AdsIP()
	aclient := "adsadapter-client"
	target := path.Join(cfg.FileDeliver.RemoteADSAdapterClientDir, aclient)
	copy, err := ktsimul.RemoteCopyIfNotExists(cfg, aclient, adsIP, target)
	if err != nil {
		log.Fatalf("failed to remote-copy, %v", err)
	}
	if copy {
		cilog.Infof("remote copied %v to %v", aclient, adsIP)
	}

	for _, file := range cfg.FileDeliver.SourceFiles {
		target := path.Join(cfg.FileDeliver.RemoteSourceFileDir, path.Base(file))
		copy, err := ktsimul.RemoteCopyIfNotExists(cfg, file, adsIP, target)
		if err != nil {
			log.Fatalf("failed to remote-copy, %v", err)
		}
		if copy {
			cilog.Infof("remote copied %v to %v", file, adsIP)
		}
	}

	hclient := "hbdeliver-client"
	target = path.Join(cfg.HBDeliver.RemoteHBClientDir, hclient)
	copy, err = ktsimul.RemoteCopyIfNotExists(cfg, hclient, cfg.HBDeliver.InstallerIP, target)
	if err != nil {
		log.Fatalf("failed to remote-copy, %v", err)
	}
	if copy {
		cilog.Infof("remote copied %v to %v", hclient, cfg.HBDeliver.InstallerIP)
	}

	for _, file := range cfg.HBDeliver.SourceFiles {
		target := path.Join(cfg.HBDeliver.RemoteSourceFileDir, path.Base(file))
		copy, err := ktsimul.RemoteCopyIfNotExists(cfg, file, cfg.HBDeliver.InstallerIP, target)
		if err != nil {
			log.Fatalf("failed to remote-copy, %v", err)
		}
		if copy {
			cilog.Infof("remote copied %v to %v", file, adsIP)
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
		<-time.After(cfg.FileDeliver.Sleep)
		err := ktsimul.RunDeliverOne(cfg, stat)
		if err != nil {
			cilog.Errorf("failed to deliver, %v", err)
		}
	}
}

func processHBDelivery(cfg *ktsimul.Config, stat *ktsimul.ProcessingStat) {
	for {
		<-time.After(cfg.HBDeliver.Sleep)
		err := ktsimul.RunHBDeliverOne(cfg, stat)
		if err != nil {
			cilog.Errorf("failed to holdback0 deliver, %v", err)
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

func processDeleteFile(cfg *ktsimul.Config, stat *ktsimul.ProcessingStat) {
	for {
		<-time.After(cfg.Delete.Sleep)
		err := ktsimul.RunDeleteOne(cfg, stat)
		if err != nil {
			cilog.Errorf("failed to delete file, %v", err)
		}
	}
}
