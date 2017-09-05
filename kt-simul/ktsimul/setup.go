package ktsimul

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"path"
	"strconv"
	"time"
)

// RunSetupOne :
func RunSetupOne(cfg *Config, localCfg LocalConfig) error {
	file, err := fileForSetup(cfg.DBAddr, cfg.DBName, cfg.DBUser, cfg.DBPass)
	if err != nil {
		return fmt.Errorf("failed to select file for setup, %v", err)
	}
	glbIP := localCfg.GLBIP
	if isHotForSetup() == false {
		glbIP = cfg.CenterGLBIPs[rand.Intn(len(cfg.CenterGLBIPs))]
	}
	client := cfg.VODClientBins[rand.Intn(len(cfg.VODClientBins))]
	isTCP := (rand.Intn(10) == 0)
	err = SetupOne(cfg, vodClientIPForSetup(cfg), client, file, glbIP, isTCP, localCfg.SessionDu)
	if err != nil {
		return fmt.Errorf("failed to setup one, %v", err)
	}
	return nil
}

// SetupOne :
func SetupOne(cfg *Config, clientIP, clientBin, file, glbIP string, isTCP bool, sessionD time.Duration) error {
	var layout = "20060102150405235"
	t := time.Now().Format(layout)

	glbPort := "1554"
	glbAddr := glbIP + ":" + glbPort

	dir := cfg.RemoteVodClientDir
	logDir := path.Join(dir, "log")
	logPath := path.Join(logDir, t+"."+file+".log")

	cmd := "mkdir -p " + logDir
	out, err := RemoteRun(clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run, %v", err)
	}

	targetBin := path.Join(dir, clientBin)
	cmd = `if [ ! -e ` + targetBin + ` ]; then echo "not exists"; fi`

	out, err = RemoteRun(clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run, %v", err)
	}
	if out != "" {
		err := RemoteCopy(clientIP, cfg.RemoteUser, cfg.RemotePass, clientBin, dir)
		if err != nil {
			return fmt.Errorf("failed to remote-run, %v", err)
		}
	}
	protocol := "cirtsp"
	if isTCP {
		protocol = "cirtspt"
	}
	playSec := sessionD.Seconds()
	if playSec == 0 {
		playSec = 1
	}
	cmd = targetBin + " " + protocol + "://" + glbAddr + "/" + file + " " + strconv.FormatInt(int64(playSec), 10) + " > " + logPath
	out, err = RemoteRun(clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return fmt.Errorf("failed to remote-run, %v", err)
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
		setupHotQueue = []bool{true, false, false, true, false, false, true, false, false, false}
	}
	ret := setupHotQueue[setupHotQueueCurIdx]
	setupHotQueueCurIdx++
	if setupHotQueueCurIdx >= len(setupHotQueue) {
		setupHotQueueCurIdx = 0
	}
	return ret
}

func fileForSetup(dbAddr, dbName, dbUser, dbPass string) (string, error) {
	file := ""
	if setupHotFileCurIdx >= len(setupHotFileQueue) || setupColdFileCurIdx >= len(setupColdFileQueue) {
		err := updateFilesForSetup(dbAddr, dbName, dbUser, dbPass)
		if err != nil {
			return "", err
		}
	}
	if isHotForSetup() {
		file = setupHotFileQueue[setupHotFileCurIdx]
		setupHotFileCurIdx++
	} else {
		file = setupColdFileQueue[setupColdFileCurIdx]
		setupColdFileCurIdx++
	}

	return file, nil
}

func vodClientIPForSetup(cfg *Config) string {
	if len(cfg.VODClientIPs) == 0 {
		log.Fatal("vod client ip is empty")
	}
	return cfg.VODClientIPs[rand.Intn(len(cfg.VODClientIPs))]
}
