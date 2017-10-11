package ktsimul

import (
	"fmt"
	"strings"

	"github.com/paulstuart/ping"
)

const getVodScript = `NUM=$(cat /usr/local/castis/LoadBalancer2.cfg|grep -v '#'| grep The_Number_Of_Servers|cut -d'=' -f2);for IDX in $(seq 0 $((NUM-1))); do grep "Server"$IDX"_Address" /usr/local/castis/LoadBalancer2.cfg|grep -v '#'; done |cut -d'=' -f2`

const centerLsmProcessCount = 11
const centerLsmProcessCountScript = `ps ax | grep castis | grep -v ssh | egrep "LFMSinkModule$|ServiceMonitor|NetIOServer3$|CiGLBServer$|LoadBalancer2$|L_GSDM$|V_GSDM$|L_NRM$|L_ADS$|ADSClient$|ADSController$" -c`

const localLsmProcessCount = 12
const localLsmProcessCountScript = `ps ax | grep castis | grep -v ssh | egrep "LFMServer$|file-requester$|LFMSinkModule$|ServiceMonitor|NetIOServer3$|CiGLBServer$|LoadBalancer2$|L_GSDM$|L_NRM$|L_ADS$|ADSClient$|ADSController$" -c`

const vodProcessCount = 6
const vodProcessCountScript = `ps ax | grep castis | grep -v ssh | egrep "ServiceMonitor|LogCollect|NetIOServer3$|ADS$|CiVODServer$|ADM$" -c`

// TestCfgIPs :
func TestCfgIPs(cfg *Config) (result string, okIPs []string) {
	var ips []string
	ips = append(ips, cfg.LsmIPs()...)
	ips = append(ips, cfg.FileDeliver.AdsIP())
	ips = append(ips, cfg.Delete.AdsIP())
	ips = append(ips, cfg.HBDeliver.InstallerIP)
	ips = append(ips, cfg.EtcIPs...)

	okList, err := TestIPs(cfg, ips)

	result = fmt.Sprintf("ip list: %v\n\n", ips)
	if err != nil {
		result += "ip check fail:\n" + err.Error() + "\n"
	} else {
		result += "all success\n"
	}
	okIPs = okList
	return
}

// TestIP :
func TestIP(cfg *Config, ip string) error {
	timeoutSec := 1
	if ping.Ping(ip, timeoutSec) {
		cmd := "uname -a"
		_, err := RemoteRun(ip, cfg.RemoteUser, cfg.RemotePass, cmd)
		if err != nil {
			return fmt.Errorf("ssh error, %v", err)
		}
		return nil
	}
	return fmt.Errorf("ping error")
}

// TestIPs :
func TestIPs(cfg *Config, ips []string) (okList []string, err error) {
	errMsg := ""
	for _, v := range ips {
		err := TestIP(cfg, v)
		if err != nil {
			errMsg += fmt.Sprintf("%v(%v)\n", v, err)
		} else {
			okList = append(okList, v)
		}
	}

	if errMsg != "" {
		return okList, fmt.Errorf("%v", errMsg)
	}
	return okList, nil
}

// TestLSMs :
func TestLSMs(cfg *Config, ipOKlist []string) string {
	lsmIPs := cfg.LsmIPs()
	msg := ""

	for _, v := range lsmIPs {
		for _, vv := range ipOKlist {
			if v == vv {
				vods, err := TestLSMOne(cfg, v)
				msg += fmt.Sprintf("lsm:%v, vods:%v ...\n", v, vods)
				if err != nil {
					msg += err.Error() + "\n"
				} else {
					msg += "all success\n\n"
				}
			}
		}
	}
	return msg
}

// TestLSMOne :
func TestLSMOne(cfg *Config, ip string) (vods []string, er error) {
	out, err := RemoteRun(ip, cfg.RemoteUser, cfg.RemotePass, getVodScript)
	if err != nil {
		return nil, fmt.Errorf("%v(ssh error,%v)", ip, err)
	}

	vods = strings.FieldsFunc(out, func(r rune) bool { return r == '\n' })

	_, err = TestIPs(cfg, vods)
	if err != nil {
		er = fmt.Errorf("vod ip check fail:\n%v", err)
		return
	}
	return
}

// type castisConfig struct {
// 	item []castisConfigItem
// }e

// type castisConfigItem struct {
// 	key string
// 	val string
// }

// func parseCastisConfig(data string) (*castisConfig, error) {
// 	cfg := &castisConfig{}
// 	strs := strings.FieldsFunc(data, func(r rune) bool { return r == '\n' })
// 	for _, v := range strs {
// 		if strings.Contains(v, "#") {
// 			continue
// 		}
// 		keyval := strings.FieldsFunc(v, func(r rune) bool { return r == '=' })
// 		if len(keyval) != 2 {
// 			return nil, fmt.Errorf("invalid castis config, line:%v", v)
// 		}
// 		cfg.item = append(cfg.item, castisConfigItem{
// 			key: strings.Trim(keyval[0], " \t"),
// 			val: strings.Trim(keyval[1], " \t"),
// 		})
// 	}
// 	return cfg, nil
// }
