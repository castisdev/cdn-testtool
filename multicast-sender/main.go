package main

import (
	"encoding/binary"
	"encoding/csv"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"path"

	"github.com/kardianos/osext"
)

const (
	outAddr               = "230.1.1.1:5001"
	maxSize               = 2048
	defaultChannelCfgFile = "channel.csv"
)

func main() {
	inAddr := flag.String("in-addr", "233.15.200.147:5000", "in-addr")
	inNic := flag.String("in-nic", "", "in-nic")
	outBindAddr := flag.String("out-bind-addr", ":0", "out-bind-addr")

	execDir, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal(err)
	}
	defaultChannelCfgFilePath := path.Join(execDir, defaultChannelCfgFile)
	channelCfg := flag.String("channel-cfg", defaultChannelCfgFilePath, "channel-cfg")

	flag.Parse()

	var senders []*sender
	in, err := os.Open(*channelCfg)
	if err != nil {
		log.Fatal(err)
	}
	r := csv.NewReader(in)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		ip := record[0]
		port := record[1]

		senders = append(senders, newSender(ip+":"+port, *outBindAddr))
	}

	addr, err := net.ResolveUDPAddr("udp", *inAddr)
	if err != nil {
		log.Fatal(err)
	}

	var nic *net.Interface
	if *inNic != "" {
		nic, err = net.InterfaceByName(*inNic)
		if err != nil {
			log.Fatal(err)
		}
	}

	var conn *net.UDPConn
	if addr.IP.IsMulticast() {
		conn, err = net.ListenMulticastUDP("udp", nic, addr)
	} else {
		conn, err = net.ListenUDP("udp4", addr)
	}
	if err != nil {
		log.Fatal(err)
	}

	first := true
	var last uint16
	for {
		buf := make([]byte, maxSize)
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Fatal(err)
		}
		if first {
			log.Printf("first packet received, n[%d]", n)
		}
		sequence := binary.BigEndian.Uint16(buf[2:])
		if first == false && last+1 != sequence {
			log.Printf("seq mismatched, expected %d, actual %d", last+1, sequence)
		}
		first = false
		last = sequence

		for _, s := range senders {
			s.ch <- buf[:n]
		}
	}
}

type sender struct {
	ch chan []byte
}

func newSender(mcastAddr, outBindAddr string) *sender {
	s := &sender{
		ch: make(chan []byte, 256),
	}

	laddr, err := net.ResolveUDPAddr("udp", outBindAddr)
	if err != nil {
		log.Fatal(err)
	}

	maddr, err := net.ResolveUDPAddr("udp4", mcastAddr)
	if err != nil {
		log.Fatal(err)
	}

	mconn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		log.Printf("sender started, mcast[%s]", mcastAddr)
		for {
			select {
			case buf, ok := <-s.ch:
				if ok == false {
					return
				}
				_, err := mconn.WriteToUDP(buf, maddr)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}()
	return s
}
