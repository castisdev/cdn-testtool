package remote

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	"github.com/castisdev/cilog"
	"golang.org/x/crypto/ssh"
)

var gClientMap map[string]*SSHClient
var gMaxSessionCount = 10
var gClientSession map[string]int
var gClientLock sync.RWMutex

// Init :
func Init(remoteIPs []string, user, pass string) {
	gClientLock.Lock()
	defer gClientLock.Unlock()

	gClientMap = make(map[string]*SSHClient)
	gClientSession = make(map[string]int)
	for _, ip := range remoteIPs {
		for i := 0; i < gMaxSessionCount; i++ {
			cl, err := Client(ip, user, pass)
			if err != nil {
				cilog.Failf("failed to connect remote client(%v), %v", ip, err)
				os.Exit(1)
			}
			cl.key = clientKey(ip, i)
			gClientMap[cl.key] = cl
			cilog.Debugf("init client(%v)", gClientMap[cl.key].key)
			<-time.After(time.Millisecond)
		}
	}
}

func clientKey(ip string, idx int) string {
	return fmt.Sprintf("%s-%d", ip, idx)
}

func allocClient(ip string) (*SSHClient, error) {
	gClientLock.Lock()
	defer gClientLock.Unlock()

	var client *SSHClient
	for i := 0; i < gMaxSessionCount; i++ {
		cl, ok := gClientMap[clientKey(ip, i)]
		if !ok {
			return nil, fmt.Errorf("not exists client, ip(%v) idx(%v)", ip, i)
		}
		if client == nil {
			client = cl
		}
		if gClientSession[cl.key] < gClientSession[client.key] {
			client = cl
		}
	}
	gClientSession[client.key]++
	cilog.Debugf("alloc client(%v) session(%v)", client.key, gClientSession[client.key])
	return client, nil
}

func releaseClient(cl *SSHClient) {
	gClientLock.Lock()
	defer gClientLock.Unlock()
	gClientSession[cl.key]--
}

// Run2 :
func Run2(ip, cmd string) (string, error) {
	cl, err := allocClient(ip)
	if err != nil {
		return "", err
	}
	defer releaseClient(cl)

	return cl.Run(cmd)
}

// Delete2 :
func Delete2(ip, filepath string) error {
	cl, err := allocClient(ip)
	if err != nil {
		return err
	}
	defer releaseClient(cl)

	return cl.Delete(filepath)
}

// SSHClient :
type SSHClient struct {
	client *ssh.Client
	key    string
	addr   string
	user   string
	pass   string
}

// Client :
func Client(addr, user, pass string) (*SSHClient, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	cl, err := ssh.Dial("tcp", addr+":22", config)
	if err != nil {
		// retry
		<-time.After(100 * time.Millisecond)
		cl, err = ssh.Dial("tcp", addr+":22", config)
		if err != nil {
			return nil, err
		}
	}
	return &SSHClient{
		client: cl,
		addr:   addr,
		user:   user,
		pass:   pass,
	}, nil
}

// Close :
func (c SSHClient) Close() {
	c.client.Close()
}

// Run :
func (c SSHClient) Run(cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	cilog.Debugf("remote-run [%v] %v", c.addr, cmd)
	err = session.Run(cmd)
	if err != nil {
		cilog.Debugf("remote-run [%v] %v", c.addr, stderr.String())
		return stdout.String(), fmt.Errorf("%v, stderr:%s", err, stderr.String())
	}
	cilog.Debugf("remote-run [%v] %v", c.addr, stdout.String())

	return stdout.String(), err
}

// Delete :
func (c SSHClient) Delete(filepath string) error {
	cmd := "rm " + filepath
	_, err := c.Run(cmd)
	return err
}

// Run :
func Run(addr, user, pass, cmd string) (string, error) {
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

// Delete :
func Delete(clientIP, user, pass, filepath string) error {
	cmd := "rm " + filepath
	_, err := Run(clientIP, user, pass, cmd)
	return err
}

// Copy :
func Copy(addr, user, pass, filepath, remoteDir string) error {
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

// CopyIfNotExists :
func CopyIfNotExists(user, pass, srcFilepath, destIP, destFilepath string) (copy bool, err error) {
	cmd := "mkdir -p " + path.Dir(destFilepath)
	out, err := Run(destIP, user, pass, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}

	cmd = `if [ ! -e ` + destFilepath + ` ]; then echo "not exists"; fi`
	out, err = Run(destIP, user, pass, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to remote-run %v, %v", cmd, err)
	}
	if out != "" {
		err := Copy(destIP, user, pass, srcFilepath, path.Dir(destFilepath))
		if err != nil {
			return false, fmt.Errorf("failed to remote-copy, %v", err)
		}
		return true, nil
	}
	return false, nil
}
