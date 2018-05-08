package main

import (
	"flag"
	"log"
	"net"
	"os"
	"time"
)

func main() {
	var file, addr string
	var bw int64
	flag.StringVar(&file, "file", "a.dat", "file path to send")
	flag.StringVar(&addr, "addr", "127.0.0.1:5000", "target udp address")
	flag.Int64Var(&bw, "bandwidth", 0, "bandwidth, 0 means unlimited")
	flag.Parse()

	srvAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal(err)
	}

	localAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.DialUDP("udp", localAddr, srvAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	err = conn.SetWriteBuffer(256 * 1024)
	if err != nil {
		log.Fatal(err)
	}

	in, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer in.Close()

	buf := make([]byte, 1024)
	for {
		now := time.Now()
		n, err := in.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		_, err = conn.Write(buf[0:n])
		if err != nil {
			log.Fatal(err)
		}
		sleepNano := int64(1)
		if bw > 0 {
			du := time.Since(now)
			// n = (duNano + x) * bw / 8 / 1000000000
			sleepNano = int64(n*8*1000000000)/bw - du.Nanoseconds()
		}
		<-time.After(time.Duration(sleepNano) * time.Nanosecond)
	}
}
