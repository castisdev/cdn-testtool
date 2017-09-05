package ktsimul

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"time"

	"golang.org/x/crypto/ssh"
	yaml "gopkg.in/yaml.v2"
)

// Config :
type Config struct {
	EADSIP                    string        `yaml:"eads-ip"`
	Locals                    []LocalConfig `yaml:"locals"`
	CenterGLBIPs              []string      `yaml:"center-glb-ips"`
	DBAddr                    string        `yaml:"db-addr"`
	DBName                    string        `yaml:"db-name"`
	DBUser                    string        `yaml:"db-user"`
	DBPass                    string        `yaml:"db-pass"`
	RemoteVodClientDir        string        `yaml:"remote-vod-client-dir"`
	RemoteADSAdapterClientDir string        `yaml:"remote-adsadapter-client-dir"`
	RemoteOriginFileDir       string        `yaml:"remote-origin-file-dir"`
	VODClientIPs              []string      `yaml:"vod-client-ips"`
	VODClientBins             []string      `yaml:"vod-client-bins"`
	DeliverSleep              time.Duration `yaml:"deliver-sleep"`
	RemoteUser                string        `yaml:"remote-user"`
	RemotePass                string        `yaml:"remote-pass"`
}

// LocalConfig :
type LocalConfig struct {
	GLBIP       string        `yaml:"glb-ip"`
	SetupPeriod time.Duration `yaml:"setup-period"`
	SessionDu   time.Duration `yaml:"session-duration"`
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
		log.Printf("remote-copy [%v] %v to %v\n", addr, filepath, path.Join(remoteDir, fname))
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
	var out bytes.Buffer
	session.Stdout = &out
	log.Printf("remote-run [%v] %v\n", addr, cmd)
	err = session.Run(cmd)
	log.Printf("remote-run [%v] %v\n", addr, out.String())
	return out.String(), err
}
