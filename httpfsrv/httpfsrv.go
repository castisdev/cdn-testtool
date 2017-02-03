package main

import (
	"flag"
	"log"
	"net/http"

	"fmt"
	"os"
	"path"
	"syscall"

	"strconv"

	"time"

	"github.com/castisdev/cdn/httputil"
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

func readfile(f *os.File, useDirectio bool, offset, len int64) []byte {
	//fmt.Printf("readfile: %v, offset: %v, len: %v\n", filepath, offset, len)
	var buf []byte
	if useDirectio {
		const alignSize = 4096
		//beg := offset - alignSize
		//if beg <= 0 {
		//	beg = 0
		//} else {
		//	beg = int64(beg/alignSize) * int64(alignSize)
		//}
		beg := offset

		o, err := f.Seek(beg, 0)
		if err != nil {
			fmt.Printf("error, %v\n", err)
			return nil
		}
		if o != beg {
			fmt.Printf("error, seek result(%v) != offset(%v)\n", o, beg)
			return nil
		}

		sz := int((len+alignSize)/alignSize) * alignSize
		if sz == int(int(len)+int(alignSize)) {
			sz = int(len)
		}
		buf = make([]byte, sz)
		//buf = directio.AlignedBlock(sz)
		readed, err := f.Read(buf)
		if err != nil {
			fmt.Printf("error, %v\n", err)
			return nil
		}
		newlen := int(len)
		if readed < newlen {
			newlen = readed
		}
		buf = buf[:newlen]
	} else {
		buf = make([]byte, len)
		o, err := f.Seek(offset, 0)
		if err != nil {
			fmt.Printf("error, %v\n", err)
			return nil
		}
		if o != offset {
			fmt.Printf("error, seek result(%v) != offset(%v)\n", o, offset)
			return nil
		}

		if _, err := f.Read(buf); err != nil {
			fmt.Printf("error, %v\n", err)
			return nil
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
	return buf
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
	<-time.After(time.Second)
	fpath := path.Join(directory, r.URL.Path)
	f, fi, err := openfile(fpath, useDirectIO)
	if err != nil {
		fmt.Printf("error 1 :%v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if ra := r.Header.Get("Range"); len(ra) > 0 {
		ras, err := httputil.ParseRange(ra, fi.Size())
		if err != nil {
			fmt.Printf("error 2 :%v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", ras[0].ContentRange(fi.Size()))
		w.Header().Set("Content-Length", strconv.FormatInt(ras[0].Length, 10))
		w.WriteHeader(206)

		var b []byte
		if useReadAll {
			b = readfile(f, useDirectIO, 0, fi.Size())
			b = b[ras[0].Start : ras[0].Start+ras[0].Length]
		} else {
			b = readfile(f, useDirectIO, ras[0].Start, ras[0].Length)
		}
		if b == nil {
			fmt.Printf("error 3 :%v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(b)
	} else {
		b := readfile(f, useDirectIO, 0, fi.Size())
		if b == nil {
			fmt.Printf("error 4 :%v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
		w.WriteHeader(200)
		w.Write(b)
	}

}

func handleHead(w http.ResponseWriter, r *http.Request) {
	fpath := path.Join(directory, r.URL.Path)
	f, err := os.Stat(fpath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if ra := r.Header.Get("Range"); len(ra) > 0 {
		ras, err := httputil.ParseRange(ra, f.Size())
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
	flag.Parse()

	useDirectIO = *directio
	directory = *dir
	useReadAll = *readAll

	r := mux.NewRouter()
	r.Methods("GET").HandlerFunc(handleGet)
	r.Methods("HEAD").HandlerFunc(handleHead)

	log.Fatal(http.ListenAndServe(*addr, r))
}
