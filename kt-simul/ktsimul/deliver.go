package ktsimul

import (
	"database/sql"
	"fmt"
	"math/rand"
	"path"
	"strings"
	"time"

	"github.com/castisdev/cdn-testtool/kt-simul/remote"
	"github.com/castisdev/cilog"
)

// DeliverEvent :
type DeliverEvent struct {
	clientIP  string
	clientDir string
	file      string
	logPath   string
	isHot     bool
	orgFile   string
}

func (ev DeliverEvent) String() string {
	return fmt.Sprintf("client(%v %v log:%v) %v isHot(%v) org(%v)",
		ev.clientIP, ev.clientDir, ev.logPath, ev.file, ev.isHot, ev.orgFile)
}

// ProcessingStat :
type ProcessingStat struct {
	Delivers       map[string]*DeliverEvent
	StartDeliverCh chan *DeliverEvent
	EndDeliverCh   chan string
}

// NewProcessingStat :
func NewProcessingStat() *ProcessingStat {
	return &ProcessingStat{
		Delivers:       make(map[string]*DeliverEvent),
		StartDeliverCh: make(chan *DeliverEvent, 100),
		EndDeliverCh:   make(chan string, 100),
	}
}

// Start :
func (p *ProcessingStat) Start() {
	for {
		select {
		case ev := <-p.StartDeliverCh:
			p.Delivers[ev.file] = ev
		case key := <-p.EndDeliverCh:
			delete(p.Delivers, key)
		}
	}
}

// StartDeliver :
func (p *ProcessingStat) StartDeliver(ev *DeliverEvent) {
	p.StartDeliverCh <- ev
}

// EndDeliver :
func (p *ProcessingStat) EndDeliver(ev *DeliverEvent) {
	p.EndDeliverCh <- ev.file
}

// RunDeliverOne :
func RunDeliverOne(cfg *Config, stat *ProcessingStat) error {
	org, err := OrgFileForDeliver(cfg)
	if err != nil {
		return fmt.Errorf("failed to select original file to deliver, %v", err)
	}

	var layout = "20060102150405235"
	t := time.Now().Format(layout)
	targetFile := t + ".mpg"

	dir := cfg.FileDeliver.RemoteADSAdapterClientDir
	logDir := path.Join(dir, "log")
	logPath := path.Join(logDir, t+".log")

	ev := &DeliverEvent{
		clientIP:  cfg.FileDeliver.AdsIP(),
		clientDir: cfg.FileDeliver.RemoteADSAdapterClientDir,
		file:      targetFile,
		logPath:   logPath,
		isHot:     IsHotForDeliver(),
		orgFile:   org,
	}
	cilog.Infof("start deliver : %s", ev)

	err = DeliverOne(cfg, ev)

	cilog.Infof("end deliver : %s error(%v)", ev.file, err != nil)
	if err != nil {
		cmd := "tail -5 " + ev.logPath
		out, e := remote.Run(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
		if e == nil {
			return fmt.Errorf("failed to deliver %v, %v\n%v %v\n%v", ev.file, err, ev.clientIP, ev.logPath, out)
		}
		return fmt.Errorf("failed to deliver %v, %v", ev.file, err)
	}
	if err := remote.Delete(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, ev.logPath); err != nil {
		return fmt.Errorf("failed to delete %v %v, %v", ev.clientIP, ev.logPath, err)
	}

	return nil
}

// DeliverOne :
func DeliverOne(cfg *Config, ev *DeliverEvent) error {
	nodes := ""
	for _, v := range cfg.CenterGLBIPs {
		if nodes != "" {
			nodes += ","
		}
		nodes += v
	}
	if ev.isHot {
		for _, v := range cfg.Locals {
			if nodes != "" {
				nodes += ","
			}
			nodes += v.GLBIP
		}
	}
	cmd := fmt.Sprintf("mkdir -p %v;cd %v;", path.Dir(ev.logPath), ev.clientDir)
	cmd += fmt.Sprintf("./adsadapter-client -org-file %v -target-file %v",
		ev.orgFile, ev.file)
	fc := cfg.FileDeliver
	cmd += fmt.Sprintf(" -adsadapter-addr %v -client-dir %v -mch-ip %v -mch-port %v -bandwidth %v -nodes %v",
		fc.ADSAdapterAddr, fc.ClientDir, fc.MchIP, fc.MchPort, fc.Bandwidth, nodes)

	cmd += " 2> " + ev.logPath
	out, err := remote.Run(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}

	cmd = "grep completed " + ev.logPath
	out, err = remote.Run(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}
	if out != "" {
		if strings.Contains(out, "failed") {
			return fmt.Errorf("failed to deliver, %v", out)
		}
		err := addServiceContents(cfg.DBAddr, cfg.DBName, cfg.DBUser, cfg.DBPass, ev.file, ev.isHot)
		if err != nil {
			return fmt.Errorf("failed to insert into service content table, %v", err)
		}
	}
	return nil
}

var hotQueue []bool
var hotQueueCurIdx int

// IsHotForDeliver :
func IsHotForDeliver() bool {
	if hotQueue == nil {
		hotQueue = []bool{true, false, false, true, false, false, true, false, false, false}
	}
	ret := hotQueue[hotQueueCurIdx]
	hotQueueCurIdx++
	if hotQueueCurIdx >= len(hotQueue) {
		hotQueueCurIdx = 0
	}
	return ret
}

// OrgFileForDeliver :
func OrgFileForDeliver(cfg *Config) (string, error) {
	cmd := "ls " + cfg.FileDeliver.RemoteSourceFileDir
	out, err := remote.Run(cfg.FileDeliver.AdsIP(), cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to remote-run, %v", err)
	}
	files := strings.Fields(out)
	if len(files) == 0 {
		return "", fmt.Errorf("not exists original files in %v %v", cfg.FileDeliver.AdsIP(), cfg.FileDeliver.RemoteSourceFileDir)
	}
	return path.Join(cfg.FileDeliver.RemoteSourceFileDir, files[rand.Intn(len(files))]), nil
}

func addServiceContents(dbAddr, dbName, dbUser, dbPass, file string, isHot bool) error {
	db, err := sql.Open("mysql", fmt.Sprintf("%v:%v@tcp(%v)/%v", dbUser, dbPass, dbAddr, dbName))
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("INSERT INTO service_content (file, is_hot) VALUES (?, ?);", file, isHot)
	return err
}
