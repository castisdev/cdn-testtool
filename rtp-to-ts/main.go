package main

import (
	"flag"
	"io"
	"log"
	"os"
)

func main() {
	var in, out string
	flag.StringVar(&in, "in", "in.rtp", "")
	flag.StringVar(&out, "out", "out.ts", "")
	flag.Parse()

	fin, err := os.Open(in)
	if err != nil {
		log.Fatal(err)
	}
	defer fin.Close()

	fout, err := os.Create(out)
	if err != nil {
		log.Fatal(err)
	}
	defer fout.Close()

	buf := make([]byte, 1316+12)

	for {
		n, err := fin.Read(buf)
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}

		if n > 12 {
			fout.Write(buf[12:n])
		}

		if err != nil {
			break
		}
	}
}
