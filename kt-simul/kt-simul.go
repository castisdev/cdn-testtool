package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"time"

	"golang.org/x/crypto/ssh"
)

var eADSIP = "172.16.110.202"
var hotGlbIP = "172.16.19.74"
var centerGlbIP = "172.16.161.132"
var user = "root"
var pass = "corey!$5"
var dbAddr = "172.16.232.23:3306"
var dbName = "kt_test"

func main() {
	// go setupOne(eADSIP, "201708291601072947.mpg", true)
	// <-time.After(time.Minute)
	deliverOne(false)
}

func setupOne(clientIP, file string, isHot bool) {
	var layout = "20060102150405235"
	t := time.Now().Format(layout)

	glbPort := "1554"
	glbIP := centerGlbIP
	if isHot {
		glbIP = hotGlbIP
	}
	glbAddr := glbIP + ":" + glbPort

	dir := "/home/castis/kt-simul/vodclient"
	logDir := path.Join(dir, "log")
	logPath := path.Join(logDir, t+"."+file+".log")

	cmd := "mkdir -p " + logDir
	log.Printf("cmd:%v\n", cmd)
	out, err := remoteRun(clientIP, user, pass, cmd)
	if err != nil {
		log.Println(out)
		log.Fatal(err)
	}
	log.Println(out)

	binName := "SimpleVODClient_Linux_x64_4.0.3.QR6.immediateplay"
	targetBin := path.Join(dir, binName)
	cmd = `if [ ! -e ` + targetBin + ` ]; then echo "not exists"; fi`
	log.Printf("cmd:%v\n", cmd)
	out, err = remoteRun(clientIP, user, pass, cmd)
	if err != nil {
		log.Println(out)
		log.Fatal(err)
	}
	log.Println(out)
	if out != "" {
		log.Println("copy")
		err := remoteCopy(clientIP, user, pass, binName, dir)
		if err != nil {
			log.Println(out)
			log.Fatal(err)
		}
	}
	cmd = targetBin + " cirtsp://" + glbAddr + "/" + file + " 2>&1 > " + logPath
	log.Printf("cmd:%v\n", cmd)
	out, err = remoteRun(clientIP, user, pass, cmd)
	if err != nil {
		log.Println(out)
		log.Fatal(err)
	}
	log.Println(out)
}
func deliverOne(isHot bool) {
	var layout = "20060102150405235"
	t := time.Now().Format(layout)

	dir := "/home/castis/kt-simul/adsa-client"
	logDir := path.Join(dir, "log")
	logPath := path.Join(logDir, t+".log")

	cmd := fmt.Sprintf("mkdir -p %v;cd %v;", logDir, dir)
	cmd += fmt.Sprintf("./adsadapter-client -org-file %v -target-file %v  -db-addr 172.16.232.23:3306",
		"AD.mpg", t+".mpg")
	if isHot == false {
		cmd += " -center"
	}
	cmd += " 2> " + logPath
	log.Printf("cmd: %v\n", cmd)
	out, err := remoteRun(eADSIP, user, pass, cmd)
	if err != nil {
		log.Println(out)
		log.Fatal(err)
	}
	log.Println(out)

	cmd = "grep completed " + logPath
	log.Printf("cmd: %v\n", cmd)
	out, err = remoteRun(eADSIP, user, pass, cmd)
	if err != nil {
		log.Println(out)
		log.Fatal(err)
	}
	if out != "" {
		log.Println(out)
	}
}

func remoteCopy(addr, user, pass, filepath, remoteDir string) error {
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
	}()
	if err := session.Run("/usr/bin/scp -tr " + remoteDir); err != nil {
		return err
	}
	return nil
}

func remoteRun(addr, user, pass, cmd string) (string, error) {
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

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	var out bytes.Buffer
	session.Stdout = &out
	err = session.Run(cmd)
	return out.String(), err
}
