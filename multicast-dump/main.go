package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/net/ipv4"
)

func main() {
	duration := flag.Duration("duration", 1*time.Minute, "dump duration")
	rtp := flag.Bool("rtp", false, "rtp source, false:udp")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] ip:port\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	addrstr := flag.Arg(0)

	filename := strings.Replace(addrstr, ".", "_", -1)
	filename = strings.Replace(filename, ":", "_", -1)
	dmp, err := os.Create(filename + ".ts")
	if err != nil {
		log.Fatalf("failed to create file, %v", err)
	}
	defer dmp.Close()

	ipv4addr, err := net.ResolveUDPAddr("udp4", addrstr)
	if err != nil {
		log.Fatalf("failed to resolve udp address, %v", err)
	}

	conn, err := net.ListenUDP("udp4", ipv4addr)
	if err != nil {
		fmt.Printf("ListenUDP error %v\n", err)
		return
	}
	defer conn.Close()

	pc := ipv4.NewPacketConn(conn)

	if ipv4addr.IP.IsMulticast() {
		ifs, err := net.Interfaces()
		if err != nil {
			log.Fatalf("failed to get interfaces, %v", err)
		}
		for _, ifi := range ifs {
			if err := pc.JoinGroup(&ifi, ipv4addr); err != nil {
				log.Fatal(err)
			}
		}
	}

	headerSize := 0
	proto := "udp"
	if *rtp {
		proto = "rtp"
		headerSize = 12
	}
	log.Printf("start to dump, %v, %v, %v", addrstr, proto, *duration)
	buf := make([]byte, 1316+headerSize)
	for start := time.Now(); time.Now().Sub(start) <= *duration; {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			log.Fatalf("failed to read, %v", err)
		}
		_, err = dmp.Write(buf[headerSize:n])
		if err != nil {
			log.Fatalf("failed to write to file, %v", err)
		}
	}
	log.Print("end to dump, ", addrstr)
}
