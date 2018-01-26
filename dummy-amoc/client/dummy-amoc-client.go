package main

import (
	"flag"
	"io/ioutil"
	"net"
	"strconv"
	"sync"
)

import "fmt"

func send(ip string, port int, reqXMLFilepath string) {
	conn, _ := net.Dial("tcp", ip+":"+strconv.Itoa(port))
	req, err := ioutil.ReadFile(reqXMLFilepath)
	if err != nil {
		panic(err)
	}
	fmt.Printf("request ================ \n\n%s\n\n", string(req))
	fmt.Fprintf(conn, string(req))

	buf := make([]byte, 4096)
	reqLen, err := conn.Read(buf)
	if err != nil {
		panic(err)
	}
	fmt.Printf("response(length:%d) ================ \n%s\n\n", reqLen, string(buf))
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "amoc ip")
	po := flag.Int("port", 3333, "amoc port")
	threadCount := flag.Int("th", 1, "thread count")
	loopCount := flag.Int("loop", 1, "loop count")
	flag.Parse()

	var wg sync.WaitGroup
	for i := 0; i < *threadCount; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < *loopCount; j++ {
				fmt.Printf("loop index: %d\n", j)
				fmt.Print("UpdateCopyConent Request Processing........\n\n")
				send(*ip, *po, "copy_req.xml")
				fmt.Print("UpdateDeleteConent Request Processing........\n\n")
				send(*ip, *po, "delete_req.xml")
			}
			defer wg.Done()
		}()
	}
	wg.Wait()
}
