package main

import (
	"log"
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

	stat := ktsimul.NewProcessingStat()
	go stat.Start()

	for _, v := range cfg.Locals {
		go processSetupForLocal(cfg, v, stat)
	}

	processDelivery(cfg, stat)
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
