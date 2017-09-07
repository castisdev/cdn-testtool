package ktsimul

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"time"

	"github.com/castisdev/cilog"
	"golang.org/x/crypto/ssh"
	yaml "gopkg.in/yaml.v2"
)

// Config :
type Config struct {
	LogDir                    string            `yaml:"log-dir"`
	LogLevel                  cilog.Level       `yaml:"log-level"`
	EADSIP                    string            `yaml:"eads-ip"`
	Locals                    []LocalConfig     `yaml:"locals"`
	CenterGLBIPs              []string          `yaml:"center-glb-ips"`
	DBAddr                    string            `yaml:"db-addr"`
	DBName                    string            `yaml:"db-name"`
	DBUser                    string            `yaml:"db-user"`
	DBPass                    string            `yaml:"db-pass"`
	RemoteVodClientDir        string            `yaml:"remote-vod-client-dir"`
	RemoteADSAdapterClientDir string            `yaml:"remote-adsadapter-client-dir"`
	RemoteOriginFileDir       string            `yaml:"remote-origin-file-dir"`
	VODClientIPs              []string          `yaml:"vod-client-ips"`
	VODClientBins             []string          `yaml:"vod-client-bins"`
	RemoteUser                string            `yaml:"remote-user"`
	RemotePass                string            `yaml:"remote-pass"`
	SourceFiles               []string          `yaml:"source-files"`
	FileDeliver               FileDeliverConfig `yaml:"file-deliver"`
	DeliverSleep              time.Duration     `yaml:"deliver-sleep"`
}

// LocalConfig :
type LocalConfig struct {
	GLBIP       string        `yaml:"glb-ip"`
	SetupPeriod time.Duration `yaml:"setup-period"`
	SessionDu   time.Duration `yaml:"session-duration"`
	DongCode    string        `yaml:"dong-code"`
}

// FileDeliverConfig :
type FileDeliverConfig struct {
	ADSAdapterAddr string `yaml:"adsadapter-addr"`
	ClientDir      string `yaml:"client-dir"`
	MchIP          string `yaml:"mch-ip"`
	MchPort        string `yaml:"mch-port"`
	Bandwidth      string `yaml:"bandwidth"`
}

// Holdback0DeliverConfig :
type Holdback0DeliverConfig struct {
	InstallerIP string `yaml:"installer-ip"`
	ClientDir   string `yaml:"client-dir"`
	MchIP       string `yaml:"mch-ip"`
	MchPort     string `yaml:"mch-port"`
	Bandwidth   string `yaml:"bandwidth"`
}

// NewConfig :
func NewConfig(ymlPath string) (*Config, error) {
	data, err := ioutil.ReadFile(ymlPath)
	if err != nil {
		return nil, fmt.Errorf("config file read fail: %v", err)
	}
	cfg := &Config{}
	err = yaml.Unmarshal([]byte(data), &cfg)
	if err != nil {
		return nil, fmt.Errorf("yaml unmarshal error: %v", err)
	}
	if len(cfg.VODClientIPs) == 0 {
		return nil, fmt.Errorf("vod client ip is empty")
	}
	return cfg, nil
}

// RemoteCopy :
func RemoteCopy(addr, user, pass, filepath, remoteDir string) error {
	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	}
	fname := path.Base(filepath)
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", addr+":22", config)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		content := string(b)
		fmt.Fprintln(w, "C0755", len(content), fname)
		fmt.Fprint(w, content)
		fmt.Fprint(w, "\x00") // transfer end with \x00
		cilog.Debugf("remote-copy [%v] %v to %v", addr, filepath, path.Join(remoteDir, fname))
	}()
	if err := session.Run("/usr/bin/scp -tr " + remoteDir); err != nil {
		return err
	}
	return nil
}

// RemoteRun :
func RemoteRun(addr, user, pass, cmd string) (string, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", addr+":22", config)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	cilog.Debugf("remote-run [%v] %v", addr, cmd)
	err = session.Run(cmd)
	if err != nil {
		cilog.Debugf("remote-run [%v] %v", addr, stderr.String())
		return stdout.String(), fmt.Errorf("%v, stderr:%s", err, stderr.String())
	}
	cilog.Debugf("remote-run [%v] %v", addr, stdout.String())

	return stdout.String(), err
}

// RemoteDelete :
func RemoteDelete(cfg *Config, clientIP, filepath string) error {
	cmd := "rm " + filepath
	_, err := RemoteRun(clientIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	return err
}

// RemoteCopyIfNotExists :
func RemoteCopyIfNotExists(cfg *Config, srcFilepath, destIP, destFilepath string) (copy bool, err error) {
	cmd := "mkdir -p " + path.Dir(destFilepath)
	out, err := RemoteRun(destIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}

	cmd = `if [ ! -e ` + destFilepath + ` ]; then echo "not exists"; fi`
	out, err = RemoteRun(destIP, cfg.RemoteUser, cfg.RemotePass, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}
	if out != "" {
		err := RemoteCopy(destIP, cfg.RemoteUser, cfg.RemotePass, srcFilepath, path.Dir(destFilepath))
		if err != nil {
			return false, fmt.Errorf("failed to remote-copy, %v", err)
		}
		return true, nil
	}
	return false, nil
}
