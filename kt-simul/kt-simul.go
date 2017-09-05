package main

import (
	"log"
	"time"

	"github.com/castisdev/cdn-testtool/kt-simul/ktsimul"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	cfg, err := ktsimul.NewConfig("kt-simul.yml")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("started")

	for _, v := range cfg.Locals {
		go processSetupForLocal(cfg, v)
	}

	processDelivery(cfg)
}

func processDelivery(cfg *ktsimul.Config) {
	for {
		<-time.After(cfg.DeliverSleep)
		err := ktsimul.RunDeliverOne(cfg)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func processSetupForLocal(cfg *ktsimul.Config, localCfg ktsimul.LocalConfig) {
	for {
		<-time.After(localCfg.SetupPeriod)
		go func() {
			err := ktsimul.RunSetupOne(cfg, localCfg)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}
}
