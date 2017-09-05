package ktsimul

import (
	"database/sql"
	"fmt"
	"math/rand"
	"path"
	"strings"
	"time"
)

// RunDeliverOne :
func RunDeliverOne(cfg *Config) error {
	org, err := OrgFileForDeliver(cfg)
	if err != nil {
		return fmt.Errorf("failed to select original file to deliver, %v", err)
	}
	err = DeliverOne(cfg, org, IsHotForDeliver())
	if err != nil {
		return fmt.Errorf("failed to deliver one, %v", err)
	}

	return nil
}

// DeliverOne :
func DeliverOne(cfg *Config, orgFile string, isHot bool) error {
	var layout = "20060102150405235"
	t := time.Now().Format(layout)
	targetFile := t + ".mpg"

	dir := cfg.RemoteADSAdapterClientDir
	logDir := path.Join(dir, "log")
	logPath := path.Join(logDir, t+".log")

	cmd := fmt.Sprintf("mkdir -p %v;cd %v;", logDir, dir)
	cmd += fmt.Sprintf("./adsadapter-client -org-file %v -target-file %v ",
		orgFile, targetFile)
	if isHot == false {
		cmd += " -center"
	}
	cmd += " 2> " + logPath
	out, err := RemoteRun(cfg.EADSIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run, %v", err)
	}

	cmd = "grep completed " + logPath
	out, err = RemoteRun(cfg.EADSIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run, %v", err)
	}
	if out != "" {
		if strings.Contains(out, "all failed") == false {
			err := addServiceContents(cfg.DBAddr, cfg.DBName, cfg.DBUser, cfg.DBPass, targetFile, isHot)
			if err != nil {
				return fmt.Errorf("failed to insert into service content table, %v", err)
			}
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
	cmd := "ls " + cfg.RemoteOriginFileDir
	out, err := RemoteRun(cfg.EADSIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to remote-run, %v", err)
	}
	files := strings.Fields(out)
	if len(files) == 0 {
		return "", fmt.Errorf("not exists original files in %v %v", cfg.EADSIP, cfg.RemoteOriginFileDir)
	}
	return path.Join(cfg.RemoteOriginFileDir, files[rand.Intn(len(files))]), nil
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
