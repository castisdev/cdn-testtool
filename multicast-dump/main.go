package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	duration := flag.Duration("duration", 1*time.Minute, "dump duration")
	flag.Parse()

	var wg sync.WaitGroup
	{
		wg.Add(1)
		go dump(flag.Arg(0), *duration, &wg)
	}

	wg.Wait()
}

func dump(addrstr string, duration time.Duration, wg *sync.WaitGroup) {
	defer wg.Done()

	filename := strings.Replace(addrstr, ".", "_", -1)
	filename = strings.Replace(filename, ":", "_", -1)
	dmp, err := os.Create(filename + ".ts")
	if err != nil {
		log.Fatal()
	}
	defer dmp.Close()

	addr, err := net.ResolveUDPAddr("udp", addrstr)
	if err != nil {
		log.Fatal()
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		log.Fatal()
	}
	defer conn.Close()

	log.Print("start to dump, ", addrstr)
	buf := make([]byte, 1316+12)
	for start := time.Now(); time.Now().Sub(start) <= duration; {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Fatal()
		}

		_, err = dmp.Write(buf[12:n])
		if err != nil {
			log.Fatal()
		}
	}
	log.Print("end to dump, ", addrstr)
}
