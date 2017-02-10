package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/castisdev/cdn/httputil"
)

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] [url]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)

	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	url := flag.Arg(0)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("%s %s", req.Method, req.URL)
	started := time.Now()

	cl := httputil.NewHTTPClientWithoutRedirect(0)
	resp, err := cl.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != 301 {
		log.Fatalf("response code is not 301 (%d)", resp.StatusCode)
	}
	resp.Body.Close()
	resp.Header.Get("Location")

	log.Printf("%v, elapsed : %v", resp.Status, time.Since(started))

	sessionUrl := resp.Header.Get("Location")

	const size = 524288
	for i := 0; i < 10; i++ {
		c := httputil.NewHTTPClient(0)
		req, err := http.NewRequest("GET", sessionUrl, nil)
		if err != nil {
			log.Fatal(err)
		}
		beg := strconv.Itoa(i * size)
		end := strconv.Itoa(i*size + size - 1)
		req.Header.Set("Range", "bytes="+beg+"-"+end)
		req.Header.Set("X-Castis-Raw-Data", "true")
		log.Printf("%s %s %s", req.Method, req.URL, req.Header.Get("Range"))
		resp, err := c.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode != 206 {
			log.Fatalf("response code is not 206 (%d)", resp.StatusCode)
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		if len(b) != size {
			log.Fatalf("readed len is not %d (%d)", size, len(b))
		}
		log.Printf("%s ok", req.Header.Get("Range"))
		// <-time.After(time.Second * 3)
	}
}
