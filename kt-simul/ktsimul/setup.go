package ktsimul

import (
	"database/sql"
	"fmt"
	"math/rand"
	"path"
	"strconv"
	"time"

	"github.com/castisdev/cilog"
)

// SetupEvent :
type SetupEvent struct {
	clientIP  string
	clientDir string
	clientBin string
	logPath   string
	file      string
	glbIP     string
	isHot     bool
	isTCP     bool
	sessionDu time.Duration
	dongCode  string
}

func (ev SetupEvent) String() string {
	return fmt.Sprintf("client(%v %v %v log:%v) %v glb(%v) isHot(%v) isTCP(%v) duration(%d) dongcode(%s)",
		ev.clientIP, ev.clientDir, ev.clientBin, path.Base(ev.logPath), ev.file, ev.glbIP, ev.isHot,
		ev.isTCP, int64(ev.sessionDu.Seconds()), ev.dongCode)
}

// RunSetupOne :
func RunSetupOne(cfg *Config, localCfg LocalConfig) error {
	isHot := isHotForSetup()
	file, err := fileForSetup(cfg.DBAddr, cfg.DBName, cfg.DBUser, cfg.DBPass, isHot)
	if err != nil {
		return fmt.Errorf("failed to select file for setup, %v", err)
	}
	glbIP := localCfg.GLBIP
	if isHot == false {
		glbIP = cfg.CenterGLBIPs[rand.Intn(len(cfg.CenterGLBIPs))]
	}

	var layout = "20060102150405235"
	t := time.Now().Format(layout)

	ev := &SetupEvent{
		clientIP:  vodClientIPForSetup(cfg),
		clientDir: cfg.RemoteVodClientDir,
		clientBin: cfg.VODClientBins[rand.Intn(len(cfg.VODClientBins))],
		file:      file,
		logPath:   path.Join(cfg.RemoteVodClientDir, "log", t+"."+file+".log"),
		glbIP:     glbIP,
		isHot:     isHot,
		isTCP:     (rand.Intn(10) == 0),
		sessionDu: localCfg.SessionDu,
		dongCode:  localCfg.DongCode,
	}
	cilog.Infof("start session: %s", ev)

	err = SetupOne(cfg, ev)

	cilog.Infof("end session: %s, error(%v)", path.Base(ev.logPath), err != nil)
	if err != nil {
		cmd := "cat " + ev.logPath
		out, e := RemoteRun(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
		if e == nil {
			return fmt.Errorf("failed to setup %v, %v\n%v %v\n%v", ev.file, err, ev.clientIP, ev.logPath, out)
		}
		return fmt.Errorf("failed to setup %v, %v", ev.file, err)
	}

	if err := RemoteDelete(cfg, ev.clientIP, ev.logPath); err != nil {
		return fmt.Errorf("failed to delete %v %v, %v", ev.clientIP, ev.logPath, err)
	}
	return nil
}

// SetupOne :
func SetupOne(cfg *Config, ev *SetupEvent) error {
	glbPort := "1554"
	glbAddr := ev.glbIP + ":" + glbPort

	cmd := "mkdir -p " + path.Dir(ev.logPath)
	out, err := RemoteRun(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}
	protocol := "cirtsp"
	if ev.isTCP {
		protocol = "cirtspt"
	}
	playSec := ev.sessionDu.Seconds()
	if playSec == 0 {
		playSec = 1
	}
	targetBin := path.Join(ev.clientDir, ev.clientBin)
	url := protocol + "://" + glbAddr + "/" + ev.file
	url += "?p=v1:CV000000000022878657:F:" + ev.dongCode + ":22736884240:N:S3"
	cmd = targetBin + " " + url + " " + strconv.FormatInt(int64(playSec), 10) + " > " + ev.logPath
	out, err = RemoteRun(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}

	cmd = "grep 'play(): success.' " + ev.logPath
	out, err = RemoteRun(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}
	if out == "" {
		return fmt.Errorf("failed to play vod")
	}

	return nil
}

func serviceFiles(db *sql.DB, limitN int, isHot bool) ([]string, error) {
	hotN := 0
	if isHot {
		hotN = 1
	}
	rows, err := db.Query("SELECT file FROM service_content where is_hot = ? ORDER BY RAND() LIMIT ?", hotN, limitN)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	var file string
	for rows.Next() {
		err := rows.Scan(&file)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

var setupHotFileQueue []string
var setupColdFileQueue []string
var setupHotFileCurIdx int
var setupColdFileCurIdx int

func updateFilesForSetup(dbAddr, dbName, dbUser, dbPass string) error {
	db, err := sql.Open("mysql", fmt.Sprintf("%v:%v@tcp(%v)/%v", dbUser, dbPass, dbAddr, dbName))
	if err != nil {
		return err
	}
	defer db.Close()

	hotFiles, err := serviceFiles(db, 10, true)
	if err != nil {
		return err
	}
	setupHotFileQueue = hotFiles
	setupHotFileCurIdx = 0

	coldFiles, err := serviceFiles(db, 10, false)
	if err != nil {
		return err
	}
	setupColdFileQueue = coldFiles
	setupColdFileCurIdx = 0
	return nil
}

var setupHotQueue []bool
var setupHotQueueCurIdx int

func isHotForSetup() bool {
	if setupHotQueue == nil {
		setupHotQueue = []bool{true, true, false, true, true, true, false, true, true, true}
	}
	ret := setupHotQueue[setupHotQueueCurIdx]
	setupHotQueueCurIdx++
	if setupHotQueueCurIdx >= len(setupHotQueue) {
		setupHotQueueCurIdx = 0
	}
	return ret
}

func fileForSetup(dbAddr, dbName, dbUser, dbPass string, isHot bool) (string, error) {
	file := ""
	if setupHotFileCurIdx >= len(setupHotFileQueue) || setupColdFileCurIdx >= len(setupColdFileQueue) {
		err := updateFilesForSetup(dbAddr, dbName, dbUser, dbPass)
		if err != nil {
			return "", err
		}
	}
	if isHot {
		file = setupHotFileQueue[setupHotFileCurIdx]
		setupHotFileCurIdx++
	} else {
		file = setupColdFileQueue[setupColdFileCurIdx]
		setupColdFileCurIdx++
	}

	return file, nil
}

func vodClientIPForSetup(cfg *Config) string {
	return cfg.VODClientIPs[rand.Intn(len(cfg.VODClientIPs))]
}
