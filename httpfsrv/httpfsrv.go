package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/castisdev/gcommon/hutil"
	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"
)

func openfile(filepath string, useDirectio bool) (f *os.File, fi os.FileInfo, err error) {
	flag := os.O_RDONLY
	if useDirectio {
		// flag |= syscall.O_DIRECT
	}
	f, err = os.OpenFile(filepath, flag, 0666)
	if err != nil {
		return
	}

	fi, err = f.Stat()
	if err != nil {
		return
	}
	return
}

func readfile(f *os.File, useDirectio bool, offset, len int64) ([]byte, error) {
	//fmt.Printf("readfile: %v, offset: %v, len: %v\n", filepath, offset, len)
	var buf []byte
	if useDirectio {
		const alignSize = 4096
		beg := offset / alignSize * alignSize

		o, err := f.Seek(beg, 0)
		if err != nil {
			return nil, err
		}
		if o != beg {
			return nil, fmt.Errorf("error, seek result(%v) != offset(%v)", o, beg)
		}

		newLen := len + (offset - beg)
		sz := (newLen-1)/alignSize*alignSize + alignSize
		buf = make([]byte, sz)
		//buf = directio.AlignedBlock(sz)
		readed, err := f.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to readfile, directio[%v], offset[%d], len[%d], err[%v]", useDirectio, offset, len, err)
		}
		if int64(readed) < len {
			len = int64(readed)
		}
		buf = buf[(offset - beg) : (offset-beg)+len]
	} else {
		buf = make([]byte, len)
		o, err := f.Seek(offset, 0)
		if err != nil {
			return nil, err
		}
		if o != offset {

			return nil, fmt.Errorf("error, seek result(%v) != offset(%v)", o, offset)
		}

		if _, err := f.Read(buf); err != nil {
			return nil, err
		}
	}

	///////////////////////////////
	//if n, err := h.fp.ReadAt(buf, offset); err != nil {
	//	if err == io.EOF && n > 0 {
	//		return buf, nil
	//	}
	//	h.mu.Unlock()
	//	return nil, err
	//}
	//////////////////////////////

	//if n != len {
	//	return nil, fmt.Errorf("readed(%v) != expected(%v)", n, len)
	//}
	return buf, nil
}

var useDirectIO bool
var directory string
var useReadAll bool
var useCastisOTU bool
var useZooinOTU bool
var headCode, getCode int
var disableRange bool
var noLastModified bool
var cacheControl string
var getDelay time.Duration
var headDelay time.Duration

func statusText(status int) string {
	return fmt.Sprintf("%d %s", status, http.StatusText(status))
}

func writeHeader(w http.ResponseWriter, r *http.Request, status int, extLog string) {
	w.WriteHeader(status)
	log.Printf("[%s] %s, %s", r.RemoteAddr, statusText(status), extLog)
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	log.Printf("[%s] %s %s, %v", r.RemoteAddr, r.Method, r.RequestURI, r.Header)
	begT := time.Now()
	if getDelay > 0 {
		<-time.After(getDelay)
	}
	if getCode > 0 {
		writeHeader(w, r, getCode, "")
		return
	}
	if (useCastisOTU || useZooinOTU) && len(r.URL.Query()["session-id"]) == 0 {
		id, _ := uuid.NewV4()
		w.Header().Set("Location", "http://localhost:8888"+r.RequestURI+"?session-id="+id.String())
		str := fmt.Sprintf("session-id(%v) redirectURL : %v", len(r.URL.Query()["session-id"]), r.RequestURI)
		writeHeader(w, r, http.StatusMovedPermanently, str)
		return
	}
	// sode origin 기능
	if r.URL.Path == "/dn_servlet" {
		r.URL.Path = r.URL.Query().Get("filename")
	}
	fpath := path.Join(directory, r.URL.Path)
	openBegT := time.Now()
	f, fi, err := openfile(fpath, useDirectIO)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		writeHeader(w, r, status, "failed to open, "+err.Error())
		return
	}
	defer f.Close()
	openDu := time.Since(openBegT)

	if noLastModified == false {
		w.Header().Set("Last-Modified", fi.ModTime().UTC().Format(http.TimeFormat))
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}

	var readDu time.Duration
	var status int
	if ra := r.Header.Get("Range"); len(ra) > 0 && disableRange == false {
		ras, err := hutil.ParseRange(ra, fi.Size())
		if err != nil {
			writeHeader(w, r, http.StatusInternalServerError, "failed to parse range, "+err.Error())
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", ras[0].ContentRange(fi.Size()))
		w.Header().Set("Content-Length", strconv.FormatInt(ras[0].Length, 10))

		if ims := r.Header.Get("If-Modified-Since"); ims != "" && ims == w.Header().Get("Last-Modified") {
			writeHeader(w, r, http.StatusNotModified, "")
			return
		}

		readBegT := time.Now()
		var b []byte
		if useReadAll {
			b, err = readfile(f, useDirectIO, 0, fi.Size())
			b = b[ras[0].Start : ras[0].Start+ras[0].Length]
		} else {
			b, err = readfile(f, useDirectIO, ras[0].Start, ras[0].Length)
		}
		if err != nil {
			writeHeader(w, r, http.StatusInternalServerError, "failed to readfile, "+err.Error())
			return
		}
		readDu = time.Since(readBegT)
		status = http.StatusPartialContent
		w.WriteHeader(status)
		w.Write(b)
	} else {
		readBegT := time.Now()
		b, err := readfile(f, useDirectIO, 0, fi.Size())
		if err != nil {
			writeHeader(w, r, http.StatusInternalServerError, "failed to readfile, "+err.Error())
			return
		}
		readDu = time.Since(readBegT)

		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))

		if ims := r.Header.Get("If-Modified-Since"); ims != "" && ims == w.Header().Get("Last-Modified") {
			writeHeader(w, r, http.StatusNotModified, "")
			return
		}
		status = http.StatusOK
		w.WriteHeader(status)
		w.Write(b)
	}
	log.Printf("[%s] %s elapsed:%v (fopen:%v/fread:%v)", r.RemoteAddr, statusText(status), time.Since(begT), openDu, readDu)
}

func handleHead(w http.ResponseWriter, r *http.Request) {
	log.Printf("[%s] %s %s, %v", r.RemoteAddr, r.Method, r.RequestURI, r.Header)
	if headDelay > 0 {
		<-time.After(headDelay)
	}
	if headCode > 0 {
		writeHeader(w, r, headCode, "")
		return
	}
	if useZooinOTU && len(r.URL.Query()["session-id"]) == 0 {
		id, _ := uuid.NewV4()
		w.Header().Set("Location", "http://localhost:8888"+r.RequestURI+"?session-id="+id.String())
		str := fmt.Sprintf("session-id(%v) redirectURL : %v", len(r.URL.Query()["session-id"]), r.RequestURI)
		writeHeader(w, r, http.StatusMovedPermanently, str)
		return
	}
	// sode origin 기능
	if r.URL.Path == "/dn_servlet" {
		r.URL.Path = r.URL.Query().Get("filename")
	}
	fpath := path.Join(directory, r.URL.Path)
	f, err := os.Stat(fpath)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		writeHeader(w, r, status, "failed to stat, "+err.Error())
		return
	}

	if noLastModified == false {
		w.Header().Set("Last-Modified", f.ModTime().UTC().Format(http.TimeFormat))
	}

	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}

	if ra := r.Header.Get("Range"); len(ra) > 0 && disableRange == false {
		ras, err := hutil.ParseRange(ra, f.Size())
		if err != nil {
			writeHeader(w, r, http.StatusInternalServerError, "failed to parse range, "+err.Error())
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", ras[0].ContentRange(f.Size()))
		w.Header().Set("Content-Length", strconv.FormatInt(ras[0].Length, 10))

		if ims := r.Header.Get("If-Modified-Since"); ims != "" && ims == w.Header().Get("Last-Modified") {
			writeHeader(w, r, http.StatusNotModified, "")
			return
		}

		writeHeader(w, r, http.StatusPartialContent, "")
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(f.Size(), 10))

		if ims := r.Header.Get("If-Modified-Since"); ims != "" && ims == w.Header().Get("Last-Modified") {
			writeHeader(w, r, http.StatusNotModified, "")
			return
		}

		writeHeader(w, r, http.StatusOK, "")
	}
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	log.Printf("[%s] %s %s, %v", r.RemoteAddr, r.Method, r.RequestURI, r.Header)
	if getCode > 0 {
		writeHeader(w, r, getCode, "")
		return
	}

	defer r.Body.Close()
	writeHeader(w, r, http.StatusCreated, "")
}

func main() {
	addr := flag.String("addr", ":8282", "listen address")
	directio := flag.Bool("directio", false, "use direct io")
	dir := flag.String("dir", "/data", "base directory contains service file")
	readAll := flag.Bool("read-all", false, "always read all contents")
	fdLimit := flag.Int("fd-limit", 8192, "fd limit")
	unixSocket := flag.Bool("usd", false, "use HTTP over unix domain socket")
	unixSocketFile := flag.String("usd-file", "/usr/local/castis/cache/sock1", "unix domain socket file path")
	otu := flag.Bool("castis-otu", false, "use castis-otu simulation, if ther is no session-id query param, redirect with session-id query param")
	zooinOTU := flag.Bool("zooin-otu", false, "use zooin-otu simulation, if ther is no session-id query param, redirect with session-id query param")
	headResp := flag.Int("head-resp", 0, "response status code about HEAD Request")
	getResp := flag.Int("get-resp", 0, "response status code about GET Request")
	disableR := flag.Bool("disable-range", false, "disable range request")
	nolm := flag.Bool("no-lm", false, "response has no Last-Modified header")
	cachecontrol := flag.String("cache-control", "", "http Cache-Control header")
	getD := flag.Duration("get-delay", 0, "response delay about GET Request")
	headD := flag.Duration("head-delay", 0, "response delay about HEAD Request")
	https := flag.Bool("https", false, "use https")
	httpsCert := flag.String("https-cert", "cert.pem", "https cert file")
	httpsKey := flag.String("https-key", "key.pem", "https key file")
	flag.Parse()

	useDirectIO = *directio
	directory = *dir
	useReadAll = *readAll
	useCastisOTU = *otu
	useZooinOTU = *zooinOTU
	headCode = *headResp
	getCode = *getResp
	disableRange = *disableR
	noLastModified = *nolm
	cacheControl = *cachecontrol
	getDelay = *getD
	headDelay = *headD

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	var rlimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit)
	if err != nil {
		log.Fatal(err)
	}
	rlimit.Max = uint64(*fdLimit)
	rlimit.Cur = uint64(*fdLimit)
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit)
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()
	r.Methods("GET").HandlerFunc(handleGet)
	r.Methods("HEAD").HandlerFunc(handleHead)
	r.Methods("POST").HandlerFunc(handlePost)

	if *unixSocket {
		if err := os.RemoveAll(*unixSocketFile); err != nil {
			log.Fatal(err)
		}
		listener, err := net.Listen("unix", *unixSocketFile)
		if err != nil {
			log.Fatal(err)
		}
		server := http.Server{
			Handler: r,
		}
		server.Serve(listener)
	} else if *https {
		log.Fatal(http.ListenAndServeTLS(*addr, *httpsCert, *httpsKey, r))
	} else {
		log.Fatal(http.ListenAndServe(*addr, r))
	}
}
