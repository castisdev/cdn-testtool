package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

func main() {
	target := flag.String("addr", ":8080", "cache server address")
	host := flag.String("host", "eve", "host")
	flag.Parse()

	for i := 1; i <= 10000; i++ {
		f := strconv.Itoa(i) + ".mpg"
		req, err := http.NewRequest("GET", "http://"+*target+"/"+f, nil)
		req.Host = *host
		if err != nil {
			log.Fatal(err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		b, _ := ioutil.ReadAll(resp.Body)
		log.Printf("%v, len[%d]", resp, len(b))
		resp.Body.Close()
	}
}
