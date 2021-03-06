package main

import (
	"bufio"
	"encoding/csv"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/castisdev/cdn-testtool/common"
	yaml "gopkg.in/yaml.v2"
)

// adsadapter message : 0x02 - body - 0x03
const (
	StartChar = byte(0x02)
	EndChar   = byte(0x03)
)

func main() {
	var orgFile, targetFile, adsaAddr, clientDir, mchIP, mchPort, nodes, delFile string
	var bandwidth int64
	flag.StringVar(&orgFile, "org-file", "org.mpg", "original file to deliver")
	flag.StringVar(&targetFile, "target-file", "", "target file name")
	flag.StringVar(&adsaAddr, "adsadapter-addr", "127.0.0.1:8083", "ADSAdapter listen address")
	flag.StringVar(&clientDir, "client-dir", "/data2/upload2", "target directory")
	flag.StringVar(&mchIP, "mch-ip", "239.0.1.11", "multicast channel ip")
	flag.StringVar(&mchPort, "mch-port", "5011", "multicast channel port")
	flag.Int64Var(&bandwidth, "bandwidth", 50000000, "bandwidth")
	flag.StringVar(&nodes, "nodes", "", "node ip list (ex)172.16.3.1,172.16.3.2")
	flag.StringVar(&delFile, "del-file", "", "file name to delete, needs adsadapter-addr/client-dir/nodes")
	flag.Parse()

	r := csv.NewReader(strings.NewReader(nodes))
	records, err := r.Read()
	if err != nil {
		log.Fatalf("failed to parse nodes option %v, %v", nodes, err)
	}

	if delFile != "" {
		err := DeleteFile(adsaAddr, delFile, clientDir, records)
		if err != nil {
			log.Printf("failed to delete file %v, %v", delFile, err)
		} else {
			log.Printf("success to delete %v", delFile)
		}
		os.Exit(0)
	}

	cfg := &Config{
		AdsadapterAddr:       adsaAddr,
		Bandwidth:            bandwidth,
		ClientDir:            clientDir,
		MulticastChannelIP:   mchIP,
		MulticastChannelPort: mchPort,
	}
	cfg.Node = records

	begT := time.Now()
	assetID := DetailedTimeToStr(time.Now())
	file := assetID + ".mpg"
	if targetFile != "" {
		file = targetFile
		if filepath.Ext(file) != "" {
			assetID = file[:len(file)-len(filepath.Ext(file))]
		} else {
			assetID = file
		}
	}
	reply, srcFile, err := FileTransfer(cfg, orgFile, assetID, file)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("received (%v): %v\n", time.Since(begT), reply)
	trid := TransactionID(reply)

	var finalErr error
	for {
		<-time.After(5 * time.Second)

		b := time.Now()
		status, err := FileTransferStatus(cfg.AdsadapterAddr, trid)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("received (%v): %v\n", time.Since(b), status)

		if ok, err := checkFinish(status); ok {
			finalErr = err
			break
		}
	}
	if finalErr != nil {
		log.Printf("%v completed with failed node, elapsed:%v, %v", file, time.Since(begT), finalErr)
	} else {
		log.Printf("%v completed, elapsed:%v", file, time.Since(begT))
	}
	if err := os.Remove(srcFile); err != nil {
		log.Printf("failed to remove %v, %v\n", srcFile, err)
	}
}

func sendXML(addr, xml string) (string, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("failed to dial %v, %v", addr, err)
	}

	_, err = conn.Write([]byte(xml))
	if err != nil {
		return "", fmt.Errorf("failed to write body to server, %v", err)
	}

	_, err = conn.Write([]byte{EndChar})
	if err != nil {
		return "", fmt.Errorf("failed to write end char to server, %v", err)
	}

	log.Printf("sended: %s\n", xml)

	reply, err := bufio.NewReader(conn).ReadSlice(EndChar)
	if err != nil {
		return "", fmt.Errorf("failed to read reply from server, %v", err)
	}
	return string(reply[1 : len(reply)-1]), nil
}

// FileTransfer :
func FileTransfer(cfg *Config, orgFile, assetID, file string) (reply string, srcFile string, e error) {
	curDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", "", err
	}
	mpgDir := filepath.Join(curDir, "transfer")
	if _, err := os.Stat(mpgDir); os.IsNotExist(err) {
		os.MkdirAll(mpgDir, 0755)
	}
	if err = common.CopyFile(filepath.Join(mpgDir, file), orgFile); err != nil {
		return "", "", err
	}
	fi, err := os.Stat(filepath.Join(mpgDir, file))
	if err != nil {
		return "", "", err
	}

	xml := `<?xml version="1.0" encoding="UTF-8"?>
<eADS>
<Job>
<Job_Name>adsadapter-client FileTransfer</Job_Name>
<Command_Date>DATETIME</Command_Date>
<Command Type="FileTransfer">
<Command_Data Name="Transfer_Mode_Priority" Value="1" /> 
<Command_Data Name="Transfer_Type" Value="1To1" /> 
<Command_Data Name="Multicast_Channel_IP" Value="MULTICAST_IP" /> 
<Command_Data Name="Multicast_Channel_Port" Value="MULTICAST_PORT" /> 
<Command_Data Name="Transfer_Speed" Value="BANDWIDTH" />
<Command_Data Name="Asset_ID" Value="ASSET_ID"/>
<Command_Data Name="File_Type" Value="Normal"/>
<Command_Data Name="File_Name" Value="FILE_NAME"/>
<Command_Data Name="File_Size" Value="FILE_SIZE"/>
<Command_Data Name="Do_Over_write" Value="YES"/>
<Command_Data Name="Server_Directory" Value="SERVER_DIR"/>
<Command_Data Name="Client_Directory" Value="CLIENT_DIR"/>
NODE_INFOS
</Command>
</Job>
</eADS>
`

	xml = strings.Replace(xml, "DATETIME", TimeToStr(time.Now()), -1)
	xml = strings.Replace(xml, "MULTICAST_IP", cfg.MulticastChannelIP, -1)
	xml = strings.Replace(xml, "MULTICAST_PORT", cfg.MulticastChannelPort, -1)
	xml = strings.Replace(xml, "BANDWIDTH", strconv.FormatInt(cfg.Bandwidth, 10), -1)
	xml = strings.Replace(xml, "ASSET_ID", assetID, -1)
	xml = strings.Replace(xml, "FILE_NAME", file, -1)
	xml = strings.Replace(xml, "FILE_SIZE", strconv.FormatInt(fi.Size(), 10), -1)
	xml = strings.Replace(xml, "SERVER_DIR", mpgDir, -1)
	xml = strings.Replace(xml, "CLIENT_DIR", cfg.ClientDir, -1)

	nodes := ""
	for _, v := range cfg.Node {
		nodes += `<Node_Information ADC_IP="` + v + `" LSM_IP="` + v + `"/>` + "\n"
	}
	if nodes == "" {
		return "", "", fmt.Errorf("not exist node info, check center-ips/node-ips config")
	}

	xml = strings.Replace(xml, "NODE_INFOS", nodes, -1)

	r, err := sendXML(cfg.AdsadapterAddr, xml)
	return r, path.Join(mpgDir, file), err
}

// TimeToStr :
func TimeToStr(t time.Time) string {
	var layout = "2006-01-02 15:04:05"
	return t.Format(layout)
}

// DetailedTimeToStr :
func DetailedTimeToStr(t time.Time) string {
	var layout = "20060102150405235"
	return t.Format(layout)
}

// FileTransferStatus :
func FileTransferStatus(addr, trid string) (string, error) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<eADS>
	<Job>
		<Job_Name>adsadapter-client FileTransferStatus</Job_Name>
		<Command_Date>DATETIME</Command_Date>
		<Command Type="FileTransferStatus">
			<Command_Data Name="Transaction_ID" Value="TRID" /> 
		</Command>
	</Job>
</eADS>
`
	xml = strings.Replace(xml, "DATETIME", TimeToStr(time.Now()), -1)
	xml = strings.Replace(xml, "TRID", trid, -1)
	return sendXML(addr, xml)
}

// DeliverError :
type DeliverError struct {
	msg            string
	isPartialError bool
}

func (e *DeliverError) Error() string { return e.msg }

func isPartialError(e error) bool {
	if e == nil {
		return false
	}
	err, ok := e.(*DeliverError)
	if ok {
		return err.isPartialError
	}
	return false
}

func checkFinish(statusXML string) (bool, error) {
	// Ready, Deploying, DeployFailed, DeployFinished
	lines := strings.FieldsFunc(statusXML, func(c rune) bool { return c == '\n' })
	finish := true
	nodeN := 0
	errMsg := ""
	errNodeN := 0
	for _, line := range lines {
		if strings.Contains(line, "Node_Information") {
			nodeN++
			fs := strings.FieldsFunc(line, func(c rune) bool { return c == '"' })
			ip := ""
			status := ""
			errCode := ""
			errString := ""
			progressRate := ""
			for i, f := range fs {
				if strings.Contains(f, "ADC_IP=") {
					ip = fs[i+1]
				}
				if strings.Contains(f, "Status=") {
					status = fs[i+1]
				}
				if strings.Contains(f, "Error_Code=") {
					errCode = fs[i+1]
				}
				if strings.Contains(f, "Error_String=") {
					errString = fs[i+1]
				}
				if strings.Contains(f, "Progress_Rate=") {
					progressRate = fs[i+1]
				}
			}
			if status != "DeployFinished" && status != "DeployFailed" {
				finish = false
			}
			if status == "DeployFailed" {
				if errMsg == "" {
					errMsg += fmt.Sprintf("%v:%v:%v:%v:progress-%v%%", ip, status, errCode, errString, progressRate)
				} else {
					errMsg += fmt.Sprintf(",%v:%v:%v:%v:progress-%v%%", ip, status, errCode, errString, progressRate)
				}
				errNodeN++
			}
		}
	}
	if errMsg != "" {
		if errNodeN == nodeN {
			return finish, &DeliverError{msg: fmt.Sprintf("all failed, %v", errMsg), isPartialError: false}
		}
		return finish, &DeliverError{msg: fmt.Sprintf("partial failed, %v", errMsg), isPartialError: true}
	}
	return finish, nil
}

// TransactionID :
func TransactionID(reply string) string {
	lines := strings.FieldsFunc(reply, func(c rune) bool { return c == '\n' })
	trid := ""
	for _, line := range lines {
		if strings.Contains(line, "Transaction_ID") {
			fs := strings.FieldsFunc(line, func(c rune) bool { return c == '"' })
			for i, f := range fs {
				if strings.Contains(f, "Value=") {
					trid = fs[i+1]
					break
				}
			}
		}
	}
	return trid
}

// DeleteFile :
func DeleteFile(adsaAddr, fname, clientDir string, nodes []string) error {
	asset := fname[:len(fname)-len(filepath.Ext(fname))]
	nodesStr := ""
	for _, v := range nodes {
		nodesStr += fmt.Sprintf(`<Node_Information ADC_IP="%s" LSM_IP="%s" />`, v, v) + "\n"
	}

	frame := `<?xml version="1.0" encoding="euc-kr" standalone="yes"?>
<eADS>
<Job>
<Job_Name>adsadapter-client DeteFileInClient</Job_Name>
<Command_Date>2017-05-05</Command_Date>
<Command Type="DeleteFileInClient">
<Command_Data Name="Asset_ID" Value="%s"></Command_Data>
<Command_Data Name="File_Name" Value="%s"></Command_Data>
<Command_Data Name="Client_Directory" Value="%s"></Command_Data>
%s
</Command>
</Job>
</eADS>
`
	reqXML := fmt.Sprintf(frame, asset, fname, clientDir, nodesStr)
	res, err := sendXML(adsaAddr, reqXML)
	if err != nil {
		return err
	}
	fmt.Println(res)

	type Response struct {
		XMLName xml.Name `xml:"eADS"`
		Job     struct {
			Report struct {
				Node []struct {
					AdcIP   string `xml:"ADC_IP,attr"`
					LsmIP   string `xml:"LSM_IP,attr"`
					ErrCode string `xml:"Error_Code,attr"`
					ErrStr  string `xml:"Error_String,attr"`
					Status  string `xml:"Status,attr"`
				} `xml:"Node_Information"`
			} `xml:"Report"`
		} `xml:"Job"`
	}
	var resXML Response
	err = xml.Unmarshal([]byte(res), &resXML)
	if err != nil {
		log.Fatalf("failed to unmarshal response xml, %v", err)
	}

	statusMsg := ""
	existsErr := false
	for _, v := range resXML.Job.Report.Node {
		statusMsg += fmt.Sprintf(", %s(%s,%s)", v.AdcIP, v.Status, v.ErrStr)
		if v.Status != "FileDeleted" {
			existsErr = true
		}
	}
	if existsErr {
		return fmt.Errorf("failed to delete %v%v", fname, statusMsg)
	}
	return nil
}

// Config :
type Config struct {
	AdsadapterAddr       string   `yaml:"adsadapter-addr"`
	Node                 []string `yaml:"node-ips"`
	Bandwidth            int64    `yaml:"bandwidth"`
	ClientDir            string   `yaml:"client-dir"`
	MulticastChannelIP   string   `yaml:"multicast-channel-ip"`
	MulticastChannelPort string   `yaml:"multicast-channel-port"`
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
	if len(cfg.Node) == 0 {
		return nil, fmt.Errorf("node-ips setting not exists")
	}
	return cfg, nil
}
