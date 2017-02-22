package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync/atomic"
	"text/template"
	"time"
)

var responseDelayMS int
var missRate int
var advIdx uint32
var hitIdx uint32
var missIdx uint32

func main() {
	listenAddr := flag.String("listen-addr", "0.0.0.0:50001", "main listen address (ex) localhost:50001")
	flag.IntVar(&responseDelayMS, "response-delay-ms", 0, "response delay millisecond (ex) 0")
	flag.IntVar(&missRate, "cache-miss-rate", 0, "cache miss rate (percent)")
	flag.Parse()

	http.HandleFunc("/", MyHandler)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}

// AD-1-N.mpg / AD-2-N.mpg : N: 1 ~ 100
func advFileForHit() string {
	idx := atomic.AddUint32(&hitIdx, 1)
	idx %= 200
	if idx%2 == 0 {
		return fmt.Sprintf("AD-1-%d.mpg", (idx/2)+1)
	}
	return fmt.Sprintf("AD-2-%d.mpg", (idx/2)+1)
}

// AD-1-miss-N.mpg / AD-2-miss-N.mpg : N: 1 ~ 500
func advFileForMiss() string {
	idx := atomic.AddUint32(&missIdx, 1)
	idx %= 1000
	if idx%2 == 0 {
		return fmt.Sprintf("AD-1-miss-%d.mpg", (idx/2)+1)
	}
	return fmt.Sprintf("AD-2-miss-%d.mpg", (idx/2)+1)
}

func MyHandler(w http.ResponseWriter, r *http.Request) {
	idx := atomic.AddUint32(&advIdx, 1)

	var advFile string
	if missRate > 0 && idx%uint32(100/missRate) == 0 {
		advFile = advFileForMiss()
	} else {
		advFile = advFileForHit()
	}

	defer r.Body.Close()
	body, _ := ioutil.ReadAll(r.Body)
	log.Printf("[%s] %s %s, %s", r.RemoteAddr, r.Method, r.URL, body)
	if responseDelayMS != 0 {
		time.Sleep(time.Duration(responseDelayMS) * time.Millisecond)
	}
	type Msg struct {
		MessageIDRef, RequestID, ADVFile string
	}

	var msg = Msg{r.URL.Query().Get("messageId"), r.URL.Query().Get("requestId"), advFile}
	t, _ := template.ParseFiles("adds.response")
	var b bytes.Buffer
	t.Execute(&b, msg)

	code := http.StatusOK
	w.WriteHeader(code)
	w.Write(b.Bytes())
	log.Printf("[%s] %d %s, %s", r.RemoteAddr, code, http.StatusText(code), b.Bytes())
}
