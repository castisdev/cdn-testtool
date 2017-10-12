package ktsimul

import (
	"database/sql"
	"fmt"
	"math/rand"
	"path"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/castisdev/cilog"
)

// SetupEvent :
type SetupEvent struct {
	clientIP    string
	clientDir   string
	clientBin   string
	logPath     string
	file        string
	glbIP       string
	secondGlbIP string
	isHot       bool
	isTCP       bool
	sessionDu   time.Duration
	dongCode    string
}

func (ev SetupEvent) String() string {
	return fmt.Sprintf("client(%v %v %v log:%v) %v glb(%v) secondGlb(%v) isHot(%v) isTCP(%v) duration(%d) dongcode(%s)",
		ev.clientIP, ev.clientDir, ev.clientBin, path.Base(ev.logPath), ev.file, ev.glbIP, ev.secondGlbIP, ev.isHot,
		ev.isTCP, int64(ev.sessionDu.Seconds()), ev.dongCode)
}

// RunSetupOne :
func RunSetupOne(cfg *Config, localCfg LocalConfig) error {
	isHot := isHotForSetup()
	file, err := fileForSetup(cfg.DBAddr, cfg.DBName, cfg.DBUser, cfg.DBPass, isHot)
	if err != nil {
		return fmt.Errorf("failed to select file for setup, %v", err)
	}
	var glbIP, secondGlbIP string
	if isHot {
		glbIP = localCfg.GLBIP
		secondGlbIP = cfg.CenterGLBIPs[rand.Intn(len(cfg.CenterGLBIPs))]
	} else {
		glbIP = cfg.CenterGLBIPs[rand.Intn(len(cfg.CenterGLBIPs))]
	}

	var layout = "20060102150405235"
	t := time.Now().Format(layout)

	ev := &SetupEvent{
		clientIP:    vodClientIPForSetup(cfg),
		clientDir:   cfg.RemoteVodClientDir,
		clientBin:   cfg.VODClientBins[rand.Intn(len(cfg.VODClientBins))],
		file:        file,
		logPath:     path.Join(cfg.RemoteVodClientDir, "log", t+"."+file+".log"),
		glbIP:       glbIP,
		secondGlbIP: secondGlbIP,
		isHot:       isHot,
		isTCP:       (rand.Intn(10) == 0),
		sessionDu:   localCfg.SessionDu,
		dongCode:    localCfg.DongCode,
	}
	cilog.Infof("start session: %s", ev)

	err = setupOne(cfg, ev)

	if err != nil {
		cilog.Errorf("end session: %s,%v", path.Base(ev.logPath), err)
	} else {
		cilog.Infof("end session: %s, success", path.Base(ev.logPath))
	}
	return nil
}

func setupOne(cfg *Config, ev *SetupEvent) error {
	needsFailover, err := setupToGlb(cfg, ev)

	if needsFailover && ev.secondGlbIP != "" {
		cilog.Infof("failed to setup %v to first glb(%v), but try to second glb(%v)", ev.file, ev.glbIP, ev.secondGlbIP)
		ev.glbIP = ev.secondGlbIP
		_, err = setupToGlb(cfg, ev)
	}

	if err != nil {
		cmd := "cat " + ev.logPath
		out, e := RemoteRun(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
		if e == nil {
			return fmt.Errorf("failed to setup %v, %v\n%v %v\n%v", ev.file, err, ev.clientIP, ev.logPath, out)
		}
		return fmt.Errorf("failed to setup %v, %v", ev.file, err)
	}

	if err := RemoteDelete(cfg, ev.clientIP, ev.logPath); err != nil {
		cilog.Warningf("failed to delete log, %v %v, %v", ev.clientIP, ev.logPath, err)
	}
	return nil
}

func setupToGlb(cfg *Config, ev *SetupEvent) (needsFailover bool, err error) {
	glbPort := "1554"
	glbAddr := ev.glbIP + ":" + glbPort

	cmd := "mkdir -p " + path.Dir(ev.logPath)
	out, err := RemoteRun(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to remote-run %v, %v", cmd, err)
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
		return false, fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}

	cmd = "grep 'play(): success.' " + ev.logPath
	out, err = RemoteRun(ev.clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return true, fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}
	if out == "" {
		return true, fmt.Errorf("failed to play vod")
	}
	return false, nil
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

var setupHotQueue []bool
var setupHotQueueCurIdx uint32

func isHotForSetup() bool {
	if setupHotQueue == nil {
		setupHotQueue = []bool{true, true, false, true, true, true, false, true, true, true}
	}
	idx := atomic.AddUint32(&setupHotQueueCurIdx, 1) % uint32(len(setupHotQueue))
	ret := setupHotQueue[idx]
	return ret
}

var setupHotFileQueue []string
var setupColdFileQueue []string
var setupHotFileCurIdx int
var setupColdFileCurIdx int
var setupFileLock sync.RWMutex

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

func fileForSetup(dbAddr, dbName, dbUser, dbPass string, isHot bool) (string, error) {
	setupFileLock.Lock()
	defer setupFileLock.Unlock()
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
