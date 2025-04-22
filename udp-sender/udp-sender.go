package main

import (
	"encoding/binary"
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
	var isRtp, repeat bool
	flag.StringVar(&file, "file", "a.dat", "file path to send")
	flag.StringVar(&addr, "addr", "127.0.0.1:5000", "target udp address")
	flag.Int64Var(&bw, "bandwidth", 0, "bandwidth, 0 means unlimited")
	flag.Int64Var(&offset, "offset", 0, "seek offset")
	flag.BoolVar(&isRtp, "rtp", false, "rtp")
	flag.BoolVar(&repeat, "repeat", false, "repeat reached eof")
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

	buf := make([]byte, 188*7)
	_, err = in.Read(buf)
	if err != nil {
		log.Fatal(err)
	}
	if buf[188] != 0x47 || buf[188*2] != 0x47 {
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

	if isRtp {
		buf = make([]byte, len(buf)+12)
	}
	start := time.Now()
	totalWrited := int64(0)
	for seq := 0; true; seq++ {
		ptr := buf
		if isRtp {
			ptr = buf[12:]
		}
		n, err := in.Read(ptr)
		if err != nil {
			if err == io.EOF && repeat {
				_, err := in.Seek(0, io.SeekStart) // Reset to the start of the file
				if err != nil {
					log.Fatal(err)
				}
				continue // Continue to the next loop iteration
			}
			log.Fatal(err)
		}
		ptr = buf
		if isRtp {
			binary.BigEndian.PutUint16(buf[2:4], uint16(seq))
			n += 12
		}
		_, err = conn.WriteToUDP(ptr[0:n], srvAddr)
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
