package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/castisdev/cdn/httputil"
	"github.com/ncw/directio"
)

type result struct {
	key     string
	elapsed time.Duration
}

func run(ch chan result, ticker *time.Ticker) {
	totalTicked := 0
	totalReaded := 0
	totalElapsed := time.Second * 0
	maxElapsed := time.Second * 0
	filemap := make(map[string]bool)
	for {
		select {
		case r := <-ch:
			totalReaded++
			totalElapsed += r.elapsed
			if maxElapsed < r.elapsed {
				maxElapsed = r.elapsed
			}
			if _, ok := filemap[r.key]; ok {
				//fmt.Printf("duplicated file read: %v\n", r.key)
			}
			filemap[r.key] = true
		case <-ticker.C:
			totalTicked++
			fmt.Printf("max elapsed: %v, Mbps: %v\n", maxElapsed, (totalReaded * 5 * 8 / totalTicked))
		}
	}
}

func readfile(filepath string, useDirectio bool) {
	flag := os.O_RDONLY
	if useDirectio {
		flag |= syscall.O_DIRECT
	}
	f, err := os.OpenFile(filepath, flag, 0666)
	if err != nil {
		fmt.Printf("error!! %v\n", err)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		fmt.Printf("error!! %v\n", err)
		return
	}
	var buf []byte
	if useDirectio {
		const alignSize = 4096
		sz := int((fi.Size()+alignSize)/alignSize) * alignSize
		if sz == (int(fi.Size()) + int(alignSize)) {
			sz = int(fi.Size())
		}
		//buf = make([]byte, sz)
		buf = directio.AlignedBlock(sz)
	} else {
		buf = make([]byte, fi.Size())
	}

	_, err = f.Read(buf)
	if err != nil {
		fmt.Printf("error!! %v\n", err)
		return
	}
	/////////////////////
	//_, err := ioutil.ReadFile(filepath)
	//if err != nil {
	//	fmt.Printf("error!! %v\n", err)
	//}
}

func readhttp(url, host string) {
	cl := httputil.NewHTTPClient(0)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("error!! %v\n", err)
		return
	}
	if host != "" {
		req.Host = host
	}
	req.Header.Set("Connection", "Close")
	res, err := cl.Do(req)
	if err != nil {
		fmt.Printf("error!! %v\n", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		fmt.Printf("error response!! %v \n", res.Status)
	}
	ioutil.ReadAll(res.Body)
}

func main() {
	dir := flag.String("dir", "/nginx-data", "directory")
	target := flag.String("addr", "", "http target address")
	host := flag.String("host", "", "http request Host header value")
	userN := flag.Int("user", 100, "user count")
	loop := flag.Int("loop", 100000, "read count per user")
	limitT := flag.String("limit-time", "30s", "limit running time")
	contentN := flag.Int("content", 10000, "total content count, content name: 1.mpg, 2.mpg, 3.mpg,...")
	directio := flag.Bool("directio", false, "use direct io")
	flag.Parse()

	duration, err := time.ParseDuration(*limitT)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ch := make(chan result)
	ticker := time.NewTicker(time.Second)
	go run(ch, ticker)

	user := *userN
	userContentLimit := int(*contentN / user)

	for i := 1; i < user+1; i++ {
		go func(idx int) {
			base := (idx - 1) * userContentLimit
			detail := 0
			for j := 0; j < *loop; j++ {
				t := time.Now()
				detail++
				if detail > userContentLimit {
					detail = 1
					//fmt.Printf("[user:%v] rolling\n", idx)
				}
				fi := base + detail
				if *target == "" {
					f := path.Join(*dir, strconv.Itoa(fi)+".mpg")
					readfile(f, *directio)
				} else {
					url := "http://" + *target + "/" + strconv.Itoa(fi) + ".mpg"
					readhttp(url, *host)
				}
				ch <- result{
					key:     strconv.Itoa(fi) + ".mpg",
					elapsed: time.Since(t),
				}
			}
		}(i)
	}
	<-time.After(duration)
}
