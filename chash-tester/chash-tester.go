package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

func main() {
	filePath := flag.String("data-filepath", "./sample-data.dat", "sample data filepath")
	sleepTime := flag.Duration("sleep-time", time.Millisecond*200, "sleep time")
	glbIPPort := flag.String("glb-ip-port", "172.16.12.23:1554", "glb ip:port (DO NOT INCLUDE http://)")
	flag.Parse()

	f, err := os.Open(*filePath)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	hit := make(map[string]int)
	var keys []string

	s := bufio.NewScanner(f)
	for s.Scan() {
		req, err := http.NewRequest("GET", "http://"+path.Join(*glbIPPort, s.Text()), nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("%s %s\n", req.Method, req.URL.RequestURI())
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer resp.Body.Close()
		loc, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			fmt.Println(err)
			return
		}
		loc.RawQuery = ""
		fmt.Printf("%s, Location: %s\n", resp.Status, loc.String())
		if _, ok := hit[loc.Host]; ok == false {
			keys = append(keys, loc.Host)
		}
		hit[loc.Host] = hit[loc.Host] + 1

		fmt.Print("HitMap : ")
		for _, k := range keys {
			fmt.Printf("[%s, %d], ", k, hit[k])
		}
		fmt.Printf("\n\n")
		// fmt.Println(hit)
		// fmt.Println()
		time.Sleep(*sleepTime)
	}
}
