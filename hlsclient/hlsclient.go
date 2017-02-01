package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	maxChunk := flag.Int("max-chunk-count", 5, "max chunk count to request")
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	log.Printf("%v, elapsed : %v", resp.Status, time.Since(started))

	filename := req.URL.RequestURI()[1:]
	o, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}

	in := bytes.NewBuffer(b)
	_, err = io.Copy(o, in)
	if err != nil {
		log.Fatal(err)
	}
	o.Close()

	log.Printf("success to write file [%s]", filename)

	m3u8, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}

	s := bufio.NewScanner(m3u8)
	nextContent := false
	cntCount := 0
	for s.Scan() {
		line := s.Text()
		if nextContent {
			nextContent = false

			req, err := http.NewRequest("GET", line, nil)
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("%s %s", req.Method, req.URL)
			started := time.Now()

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Fatal(err)
			}

			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			resp.Body.Close()
			log.Printf("%v, elapsed : %v", resp.Status, time.Since(started))

			filename := req.URL.RequestURI()[1:]
			o, err := os.Create(filename)
			if err != nil {
				log.Fatal(err)
			}

			in := bytes.NewBuffer(b)
			_, err = io.Copy(o, in)
			if err != nil {
				log.Fatal(err)
			}
			o.Close()

			log.Printf("success to write file [%s]", filename)
			cntCount++
		}
		nextContent = strings.Contains(line, "#EXTINF")

		if cntCount == *maxChunk {
			break
		}
	}
	m3u8.Close()
}
