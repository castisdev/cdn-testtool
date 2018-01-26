package ktsimul

import (
	"fmt"
	"log"
	"math/rand"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/castisdev/cdn-testtool/kt-simul/remote"
	"github.com/castisdev/cilog"
)

// HBDeliverEvent :
type HBDeliverEvent struct {
	remoteIP string
	file     string
	orgFile  string
	channel  string
	logPath  string
}

func (ev HBDeliverEvent) String() string {
	return fmt.Sprintf("%v %v %v org(%v)",
		ev.remoteIP, ev.file, ev.channel, ev.orgFile)
}

// RunHBDeliverOne :
func RunHBDeliverOne(cfg *Config, stat *ProcessingStat) error {
	org, err := OrgFileForHBDeliver(cfg)
	if err != nil {
		return fmt.Errorf("failed to select original file to deliver, %v", err)
	}

	var layout = "20060102150405235"
	t := time.Now().Format(layout)
	targetFile := t + ".mpg"

	dir := cfg.HBDeliver.RemoteHBClientDir
	logDir := path.Join(dir, "log")
	logPath := path.Join(logDir, targetFile[:len(targetFile)-len(filepath.Ext(targetFile))]+".log")

	ev := &HBDeliverEvent{
		remoteIP: cfg.HBDeliver.InstallerIP,
		file:     targetFile,
		orgFile:  org,
		channel:  cfg.HBDeliver.Channels[rand.Intn(len(cfg.HBDeliver.Channels))],
		logPath:  logPath,
	}
	cilog.Infof("start holdback0 deliver : %s", ev)

	err = HBDeliverOne(cfg, ev)

	cilog.Infof("end holdback0 deliver : %s error(%v)", ev.file, err != nil)
	if err != nil {
		cmd := "tail -5 " + ev.logPath
		out, e := remote.Run(cfg.HBDeliver.InstallerIP, cfg.RemoteUser, cfg.RemotePass, cmd)
		if e == nil {
			return fmt.Errorf("failed to deliver %v, %v\n%v %v\n%v", ev.file, err, cfg.HBDeliver.InstallerIP, ev.logPath, out)
		}
		return fmt.Errorf("failed to deliver %v, %v", ev.file, err)
	}
	if err := remote.Delete(cfg.HBDeliver.InstallerIP, cfg.RemoteUser, cfg.RemotePass, ev.logPath); err != nil {
		return fmt.Errorf("failed to delete %v %v, %v", cfg.HBDeliver.InstallerIP, ev.logPath, err)
	}

	return nil
}

// HBDeliverOne :
func HBDeliverOne(cfg *Config, ev *HBDeliverEvent) error {
	go func() {
		<-time.After(10 * time.Second)
		isHot := true
		err := addServiceContents(cfg.DBAddr, cfg.DBName, cfg.DBUser, cfg.DBPass, ev.file, isHot)
		if err != nil {
			log.Printf("failed to insert into service content table, %v", err)
		}
	}()

	src := path.Join(cfg.HBDeliver.RemoteSourceFileDir, path.Base(ev.orgFile))

	cmd := fmt.Sprintf("mkdir -p %v;cd %v;", path.Dir(ev.logPath), cfg.HBDeliver.RemoteHBClientDir)
	cmd += "./hbdeliver-client"
	cmd += " -org-file " + src
	cmd += " -target-file " + ev.file
	cmd += " -ch " + ev.channel
	cmd += " -import-dir " + cfg.HBDeliver.ImportDir
	cmd += " -loaded-dir " + cfg.HBDeliver.LoadedDir
	cmd += " -error-dir " + cfg.HBDeliver.ErrorDir
	cmd += " 2> " + ev.logPath

	_, err := remote.Run(cfg.HBDeliver.InstallerIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run %v\n%v", cmd, err)
	}

	cmd = "grep completed " + ev.logPath
	out, err := remote.Run(cfg.HBDeliver.InstallerIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}
	if out != "" {
		if strings.Contains(out, "failed") {
			return fmt.Errorf("failed to deliver, %v", out)
		}
	}
	return nil
}

// OrgFileForHBDeliver :
func OrgFileForHBDeliver(cfg *Config) (string, error) {
	cmd := "ls " + cfg.HBDeliver.RemoteSourceFileDir
	out, err := remote.Run(cfg.HBDeliver.InstallerIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to remote-run, %v", err)
	}
	files := strings.Fields(out)
	if len(files) == 0 {
		return "", fmt.Errorf("not exists original files in %v %v", cfg.HBDeliver.InstallerIP, cfg.HBDeliver.RemoteSourceFileDir)
	}
	return path.Join(cfg.FileDeliver.RemoteSourceFileDir, files[rand.Intn(len(files))]), nil
}
