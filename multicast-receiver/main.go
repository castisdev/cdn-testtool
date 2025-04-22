package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Stream struct {
	URL string `json:"url"`
}

func main() {
	sroot := flag.String("stream-root", "", "stream-root")
	flag.Parse()

	files, err := os.ReadDir(*sroot)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}

		if file.IsDir() {
			continue
		}

		if !strings.HasSuffix(file.Name(), ".stream.bak") {
			continue
		}

		f, err := os.Open(filepath.Join(*sroot, file.Name()))
		if err != nil {
			log.Fatal(err)
		}

		d := json.NewDecoder(f)
		var stm Stream
		d.Decode(&stm)
		fmt.Println(stm)
	}
}
