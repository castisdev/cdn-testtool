package main

import (
	"flag"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

func main() {
	duration := flag.Duration("duration", 1*time.Minute, "dump duration")
	src := flag.String("src", "0.0.0.0:5000", "udp source")
	dest := flag.String("dest", "127.0.0.1:6000", "udp destination")
	lossPercent := flag.Int("loss", 0, "packet loss percent, 0[no loss], 100[random per 100 packet]")
	flag.Parse()

	var wg sync.WaitGroup
	{
		wg.Add(1)
		go relay(*src, *dest, *lossPercent, *duration, &wg)
	}

	wg.Wait()
}

func relay(src, dest string, lossPercent int, duration time.Duration, wg *sync.WaitGroup) {
	defer wg.Done()

	srcAddr, err := net.ResolveUDPAddr("udp", src)
	if err != nil {
		log.Fatal()
	}

	destAddr, err := net.ResolveUDPAddr("udp", dest)
	if err != nil {
		log.Fatal()
	}

	srcConn, err := net.ListenUDP("udp", srcAddr)
	if err != nil {
		log.Fatal()
	}
	defer srcConn.Close()

	destConn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		log.Fatal(err)
	}
	defer destConn.Close()

	log.Printf("start to relay, %v to %v\n", src, dest)

	buf := make([]byte, 1316)
	lossRandom := lossPercent == 100
	relayCnt := 0
	lossCnt := 0

	if lossRandom {
		lossPercent = 50
	}
	for start := time.Now(); time.Now().Sub(start) <= duration; {
		if s := relayCnt + lossCnt; s > 0 && s%100 == 0 {
			if lossRandom {
				lossPercent = rand.Intn(100)
			}
			log.Printf("losspercent:%v, relay:%v, loss:%v\n", lossPercent, relayCnt, lossCnt)
		}

		n, _, err := srcConn.ReadFromUDP(buf)
		if err != nil {
			log.Fatal()
		}

		loss := false
		if lossPercent > 0 {
			if rand.Intn(100) < lossPercent {
				loss = true
			}
		}

		if loss {
			lossCnt++
			continue
		}

		_, err = destConn.WriteTo(buf[:n], destAddr)
		if err != nil {
			log.Fatal(err)
		}
		relayCnt++
	}
	log.Printf("end to relay, %v to %v\n", src, dest)
}
