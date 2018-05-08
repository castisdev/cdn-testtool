package main

import (
	"flag"
	"fmt"
	"net"
	"os"
)

func checkError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(0)
	}
}

func main() {
	var addr string
	flag.StringVar(&addr, "addr", "0.0.0.0:5000", "listen udp address")
	flag.Parse()

	serverAddr, err := net.ResolveUDPAddr("udp", addr)
	checkError(err)

	conn, err := net.ListenUDP("udp", serverAddr)
	checkError(err)
	defer conn.Close()
	err = conn.SetReadBuffer(256 * 1024)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	fpath := "udp.dump"
	os.Remove(fpath)
	f, err := os.Create(fpath)
	checkError(err)
	defer f.Close()

	buf := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error: ", err)
			os.Exit(1)
		}
		_, err = f.Write(buf[:n])
		if err != nil {
			fmt.Println("Error: ", err)
			os.Exit(1)
		}
	}
}
