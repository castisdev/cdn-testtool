package main

import (
	"flag"
	"io"
	"log"
	"os"
)

func main() {
	in := flag.String("in", "", "file input, required")
	out := flag.String("out", "", "file output, required")
	offset := flag.Int64("offset", 0, "file offset, default 0")
	length := flag.Int64("size", 0, "file size, 0: [offset]-[eof], default 0")
	flag.Parse()

	if len(*in) == 0 || len(*out) == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	subfile(*in, *out, *offset, *length)
}

func subfile(in, out string, offset, length int64) {
	fi, err := os.Open(in)
	if err != nil {
		panic(err)
	}
	defer fi.Close()

	fo, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer fo.Close()

	off, err := fi.Seek(offset, 0)
	if err != nil {
		panic(err)
	}
	if off != offset {
		log.Fatalf("failed to seek %d, ret_offset:%d", offset, off)
	}

	writed := int64(0)
	buff := make([]byte, 4096)
	for {
		cnt, err := fi.Read(buff)
		if err != nil && err != io.EOF {
			panic(err)
		}
		if cnt == 0 {
			break
		}
		if cnt > int(length-writed) {
			cnt = int(length - writed)
		}
		_, err = fo.Write(buff[:cnt])
		if err != nil {
			panic(err)
		}
		writed += int64(cnt)
		if writed >= length {
			break
		}
	}
}
