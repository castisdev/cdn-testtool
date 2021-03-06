package ktsimul

import (
	"database/sql"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/castisdev/cdn-testtool/kt-simul/remote"
	"github.com/castisdev/cilog"
)

// DeleteEvent :
type DeleteEvent struct {
	clientIP  string
	clientDir string
	file      string
	logPath   string
}

func (ev DeleteEvent) String() string {
	return fmt.Sprintf("client(%v %v log:%v) %v",
		ev.clientIP, ev.clientDir, ev.logPath, ev.file)
}

// RunDeleteOne :
func RunDeleteOne(cfg *Config, stat *ProcessingStat) error {
	file, err := selectDeleteFile(cfg)
	if err != nil {
		return fmt.Errorf("failed to select file to delete, %v", err)
	}

	return DeleteOne(cfg, file)
}

// DeleteOne :
func DeleteOne(cfg *Config, file string) error {
	var layout = "20060102150405235"
	t := time.Now().Format(layout)
	dir := cfg.Delete.RemoteADSAdapterClientDir
	logDir := path.Join(dir, "log")
	logPath := path.Join(logDir, t+".delete.log")

	ev := &DeleteEvent{
		clientIP:  cfg.Delete.AdsIP(),
		clientDir: cfg.Delete.RemoteADSAdapterClientDir,
		file:      file,
		logPath:   logPath,
	}
	cilog.Infof("start delete : %s", ev)

	err := deleteOneInternal(cfg, ev)

	cilog.Infof("end delete : %s error(%v)", ev.file, err != nil)
	if err != nil {
		return fmt.Errorf("failed to delete %v, %v", ev.file, err)
	}
	if err := remote.Delete(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, ev.logPath); err != nil {
		return fmt.Errorf("failed to delete %v %v, %v", ev.clientIP, ev.logPath, err)
	}

	return nil
}

// deleteOneInternal :
func deleteOneInternal(cfg *Config, ev *DeleteEvent) error {
	nodes := ""
	for _, v := range cfg.CenterGLBIPs {
		if nodes != "" {
			nodes += ","
		}
		nodes += v
	}
	for _, v := range cfg.FrozenLSMIPs {
		if nodes != "" {
			nodes += ","
		}
		nodes += v
	}
	for _, v := range cfg.Locals {
		if nodes != "" {
			nodes += ","
		}
		nodes += v.GLBIP
	}

	cmd := fmt.Sprintf("mkdir -p %v;cd %v;", path.Dir(ev.logPath), ev.clientDir)
	cmd += fmt.Sprintf("./adsadapter-client -adsadapter-addr %v -del-file %v -client-dir %v -nodes %v",
		cfg.Delete.ADSAdapterAddr, ev.file, cfg.Delete.ClientDir, nodes)

	cmd += " 2> " + ev.logPath
	out, err := remote.Run(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}

	cmd = "tail -1 " + ev.logPath
	out, err = remote.Run(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}
	if out != "" {
		if strings.Contains(out, "success to delete") {
			err := deleteServiceContents(cfg.DBAddr, cfg.DBName, cfg.DBUser, cfg.DBPass, ev.file)
			if err != nil {
				return fmt.Errorf("failed to delete from service content table, %v", err)
			}
			return nil
		}
		return fmt.Errorf("%v", out)
	}
	return fmt.Errorf("invalid log")
}

func selectDeleteFile(cfg *Config) (string, error) {
	db, err := sql.Open("mysql", fmt.Sprintf("%v:%v@tcp(%v)/%v", cfg.DBUser, cfg.DBPass, cfg.DBAddr, cfg.DBName))
	if err != nil {
		return "", err
	}
	defer db.Close()
	rows, err := db.Query("SELECT file FROM service_content where is_hot = 0 ORDER BY RAND() LIMIT 1")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var file string
	for rows.Next() {
		err := rows.Scan(&file)
		if err != nil {
			return "", err
		}
		break
	}
	return file, nil
}

func deleteServiceContents(dbAddr, dbName, dbUser, dbPass, file string) error {
	db, err := sql.Open("mysql", fmt.Sprintf("%v:%v@tcp(%v)/%v", dbUser, dbPass, dbAddr, dbName))
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("DELETE FROM service_content WHERE file=?;", file)
	return err
}
