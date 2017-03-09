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

	"github.com/castisdev/cdn/hutil"
	"github.com/gorilla/mux"
)

func openfile(filepath string, useDirectio bool) (f *os.File, fi os.FileInfo, err error) {
	flag := os.O_RDONLY
	if useDirectio {
		flag |= syscall.O_DIRECT
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

//
//func readfile(f *os.File, useDirectio bool, start, length int64) []byte {
//	var buf []byte
//	if useDirectio {
//		const alignSize = 4096
//		sz := int((length+alignSize)/alignSize) * alignSize
//		if sz == (int(length) + int(alignSize)) {
//			sz = int(length)
//		}
//		buf = make([]byte, sz)
//		//buf = directio.AlignedBlock(int(fi.Size()))
//	} else {
//		buf = make([]byte, length)
//	}
//	if start != 0 {
//		_, err := f.ReadAt(buf, start)
//		if err != nil {
//			fmt.Printf("error!! %v\n", err)
//			return nil
//		}
//	} else {
//		_, err := f.Read(buf)
//		if err != nil {
//			fmt.Printf("error!! %v\n", err)
//			return nil
//		}
//	}
//	return buf
//}

var useDirectIO bool
var directory string
var useReadAll bool

func handleGet(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s, %v", r.Method, r.RequestURI, r.Header)
	fpath := path.Join(directory, r.URL.Path)
	f, fi, err := openfile(fpath, useDirectIO)
	if err != nil {
		log.Printf("failed to open, %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Last-Modified", fi.ModTime().Format(time.RFC1123))
	if ra := r.Header.Get("Range"); len(ra) > 0 {
		ras, err := hutil.ParseRange(ra, fi.Size())
		if err != nil {
			log.Printf("failed to parse range, %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", ras[0].ContentRange(fi.Size()))
		w.Header().Set("Content-Length", strconv.FormatInt(ras[0].Length, 10))

		var b []byte
		if useReadAll {
			b, err = readfile(f, useDirectIO, 0, fi.Size())
			b = b[ras[0].Start : ras[0].Start+ras[0].Length]
		} else {
			b, err = readfile(f, useDirectIO, ras[0].Start, ras[0].Length)
		}
		if err != nil {
			log.Printf("failed to readfile, %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(206)
		w.Write(b)
	} else {
		b, err := readfile(f, useDirectIO, 0, fi.Size())
		if err != nil {
			log.Printf("failed to readfile, %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
		w.WriteHeader(200)
		w.Write(b)
	}

}

func handleHead(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s, %v", r.Method, r.RequestURI, r.Header)
	fpath := path.Join(directory, r.URL.Path)
	f, err := os.Stat(fpath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Last-Modified", f.ModTime().Format(time.RFC1123))
	if ra := r.Header.Get("Range"); len(ra) > 0 {
		ras, err := hutil.ParseRange(ra, f.Size())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", ras[0].ContentRange(f.Size()))
		w.Header().Set("Content-Length", strconv.FormatInt(ras[0].Length, 10))
		w.WriteHeader(206)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(f.Size(), 10))
		w.WriteHeader(200)
	}
}

func main() {
	addr := flag.String("addr", ":8282", "listen address")
	directio := flag.Bool("directio", false, "use direct io")
	dir := flag.String("dir", "/nginx-data", "base directory contains service file")
	readAll := flag.Bool("read-all", false, "always read all contents")
	fdLimit := flag.Int("fd-limit", 8192, "fd limit")
	unixSocket := flag.Bool("usd", false, "use HTTP over unix domain socket")
	unixSocketFile := flag.String("usd-file", "/usr/local/castis/cache/sock1", "unix domain socket file path")
	flag.Parse()

	useDirectIO = *directio
	directory = *dir
	useReadAll = *readAll

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
	} else {
		log.Fatal(http.ListenAndServe(*addr, r))
	}
}
