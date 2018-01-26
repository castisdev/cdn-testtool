package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"syscall"
)

func runMainProc(listenAddr string, workerN int) {
	listener, err := net.Listen("tcp4", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	tcplistener := listener.(*net.TCPListener)
	lf, err := tcplistener.File()
	if err != nil {
		log.Fatalf("failed to get listen fd, %v", err)
	}
	fd := uint(lf.Fd())

	var st syscall.Stat_t
	if err := syscall.Fstat(int(fd), &st); err != nil {
		log.Fatalf("failed to fstat, %v", err)
	}
	sockStr := fmt.Sprintf("%d", st.Ino)

	var procs []*os.Process
	for i := 0; i < workerN; i++ {
		var procAttr os.ProcAttr
		procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr, lf}
		argv := []string{
			path.Base(os.Args[0]),
			"-worker-id", "worker" + strconv.Itoa(i+1),
			"-listen-sock", sockStr,
		}
		process, err := os.StartProcess(argv[0], argv,
			&procAttr)
		if err != nil {
			log.Fatalf("failed to start process, %v", err)
		}
		procs = append(procs, process)
	}
	for _, p := range procs {
		_, err = p.Wait()
		if err != nil {
			log.Fatalf("failed to wait process, %v", err)
		}
	}
}

type handler struct {
	workerID string
	msg      []byte
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write(h.msg)
	log.Printf("[%s] %s", h.workerID, h.msg)
}

func runWorker(workerID string, listenSockInode uint64) {
	listenFD := 0
	visitF := func(filepath string, f os.FileInfo, err error) error {
		fd, err := strconv.Atoi(path.Base(filepath))
		if err != nil || fd == 0 {
			return nil
		}
		var st syscall.Stat_t
		if err := syscall.Fstat(fd, &st); err != nil {
			return fmt.Errorf("failed to fstat, %v", err)
		}
		if st.Ino == listenSockInode {
			listenFD = fd
		}
		return nil
	}
	fddir := path.Join("/proc", strconv.Itoa(os.Getpid()), "fd")
	if err := filepath.Walk(fddir, visitF); err != nil {
		log.Fatalf("failed to get listen socket inode, %v", listenSockInode)
	}

	listener, err := net.FileListener(os.NewFile(uintptr(listenFD), "listen sock"))
	if err != nil {
		log.Fatal(err)
	}
	h := &handler{
		workerID: workerID,
		msg:      []byte("Hello World"),
	}
	server := http.Server{
		Handler: h,
	}
	server.Serve(listener)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("failed to serve, %s", err)
	}
}

func main() {
	var workerID, listenAddr string
	var workerN int
	var listenSockInode uint64
	flag.StringVar(&listenAddr, "listen-addr", "0.0.0.0:12345", "listen address")
	flag.IntVar(&workerN, "nworker", 2, "worker count")
	flag.StringVar(&workerID, "worker-id", "", "DO NOT USE THIS (internal option)")
	flag.Uint64Var(&listenSockInode, "listen-sock", 0, "DO NOT USE THIS (internal option)")
	flag.Parse()

	if len(workerID) == 0 {
		runMainProc(listenAddr, workerN)
	} else {
		runWorker(workerID, listenSockInode)
	}
}
