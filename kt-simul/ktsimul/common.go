package ktsimul

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/castisdev/cilog"
	yaml "gopkg.in/yaml.v2"
)

// Config :
type Config struct {
	LogDir             string                 `yaml:"log-dir"`
	LogLevel           cilog.Level            `yaml:"log-level"`
	Locals             []LocalConfig          `yaml:"locals"`
	CenterGLBIPs       []string               `yaml:"center-glb-ips"`
	FrozenLSMIPs       []string               `yaml:"frozen-lsm-ips"`
	EtcIPs             []string               `yaml:"etc-ips"`
	DBAddr             string                 `yaml:"db-addr"`
	DBName             string                 `yaml:"db-name"`
	DBUser             string                 `yaml:"db-user"`
	DBPass             string                 `yaml:"db-pass"`
	RemoteVodClientDir string                 `yaml:"remote-vod-client-dir"`
	VODClientIPs       []string               `yaml:"vod-client-ips"`
	VODClientBins      []string               `yaml:"vod-client-bins"`
	RemoteUser         string                 `yaml:"remote-user"`
	RemotePass         string                 `yaml:"remote-pass"`
	FileDeliver        FileDeliverConfig      `yaml:"file-deliver"`
	HBDeliver          Holdback0DeliverConfig `yaml:"holdback0-deliver"`
	Delete             DeleteConfig           `yaml:"delete-file"`
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
	ADSAdapterAddr            string        `yaml:"adsadapter-addr"`
	RemoteSourceFileDir       string        `yaml:"remote-source-file-dir"`
	RemoteADSAdapterClientDir string        `yaml:"remote-adsadapter-client-dir"`
	ClientDir                 string        `yaml:"client-dir"`
	MchIP                     string        `yaml:"mch-ip"`
	MchPort                   string        `yaml:"mch-port"`
	Bandwidth                 string        `yaml:"bandwidth"`
	SourceFiles               []string      `yaml:"source-files"`
	Sleep                     time.Duration `yaml:"sleep"`
}

// AdsIP :
func (f *FileDeliverConfig) AdsIP() string {
	return f.ADSAdapterAddr[0:strings.Index(f.ADSAdapterAddr, ":")]
}

// Holdback0DeliverConfig :
type Holdback0DeliverConfig struct {
	InstallerIP         string        `yaml:"assetinstaller-ip"`
	ImportDir           string        `yaml:"import-dir"`
	LoadedDir           string        `yaml:"loaded-dir"`
	ErrorDir            string        `yaml:"error-dir"`
	Channels            []string      `yaml:"channels"`
	RemoteSourceFileDir string        `yaml:"remote-source-file-dir"`
	RemoteHBClientDir   string        `yaml:"remote-hb-client-dir"`
	SourceFiles         []string      `yaml:"source-files"`
	Sleep               time.Duration `yaml:"sleep"`
}

// DeleteConfig :
type DeleteConfig struct {
	ADSAdapterAddr            string        `yaml:"adsadapter-addr"`
	RemoteADSAdapterClientDir string        `yaml:"remote-adsadapter-client-dir"`
	ClientDir                 string        `yaml:"client-dir"`
	Sleep                     time.Duration `yaml:"sleep"`
}

// AdsIP :
func (f *DeleteConfig) AdsIP() string {
	return f.ADSAdapterAddr[0:strings.Index(f.ADSAdapterAddr, ":")]
}

// LsmIPs :
func (c *Config) LsmIPs() []string {
	var lsmIPs []string
	lsmIPs = append(lsmIPs, c.CenterGLBIPs...)
	lsmIPs = append(lsmIPs, c.FrozenLSMIPs...)
	for _, v := range c.Locals {
		lsmIPs = append(lsmIPs, v.GLBIP)
	}
	return lsmIPs
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
