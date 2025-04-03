package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"time"
)

func main() {
	var file, addr string
	var bw, offset int64
	var repeat int
	var blocksize int
	flag.StringVar(&file, "file", "a.dat", "file path to send")
	flag.StringVar(&addr, "addr", "127.0.0.1:5000", "target udp address")
	flag.Int64Var(&bw, "bandwidth", 0, "bandwidth, 0 means unlimited")
	flag.Int64Var(&offset, "offset", 0, "seek offset")
	flag.IntVar(&blocksize, "block-size", 1316, "read / send block size, default 1316")
	flag.IntVar(&repeat, "repeat", 0, "repeat reached eof")
	flag.Parse()

	srvAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal(err)
	}

	localAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.ListenUDP("udp", localAddr)
	// conn, err := net.DialUDP("udp", localAddr, srvAddr)
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

	buf := make([]byte, blocksize)
	_, err = in.Read(buf)
	if err != nil {
		log.Fatal(err)
	}
	if buf[188] != 0x47 {
		if buf[204] == 0x47 {
			buf = make([]byte, 204*7)
		} else if buf[12] == 0x47 {
			buf = make([]byte, 188*7+12)
		} else {
			log.Fatal("sync byte mismatch")
		}
	}

	if offset != 0 {
		offset = offset / int64(len(buf)) * int64(len(buf))
	}

	_, err = in.Seek(offset, io.SeekStart)
	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	totalWrited := int64(0)
	for {
		n, err := in.Read(buf)
		if err != nil {
			// If EOF is reached, reset the file pointer to start again
			if err == io.EOF && repeat != 0 {
				_, err := in.Seek(0, io.SeekStart) // Reset to the start of the file
				if err != nil {
					log.Fatal(err)
				}
				continue // Continue to the next loop iteration
			}
			log.Fatal(err)
		}
		_, err = conn.WriteToUDP(buf[0:n], srvAddr)
		if err != nil {
			log.Fatal(err)
		}
		totalWrited += int64(n)
		sleepMillie := int64(1)
		if bw > 0 {
			du := time.Since(start)
			// n = (duNano + x) * bw / 8 / 1000000000
			sleepMillie = int64(totalWrited*8*1000)/bw - du.Milliseconds()
		}
		<-time.After(time.Duration(sleepMillie) * time.Millisecond)
	}
}
