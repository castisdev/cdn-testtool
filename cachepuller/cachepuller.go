package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

func main() {
	for i := 1; i <= 10000; i++ {
		f := strconv.Itoa(i) + ".mpg"
		req, err := http.NewRequest("GET", "http://localhost:8080/"+f, nil)
		req.Host = "eve"
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
