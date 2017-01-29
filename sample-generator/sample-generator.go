package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	uuid "github.com/satori/go.uuid"
)

func main() {
	fileCount := flag.Int("file-count", 1000, "file count")
	outputDir := flag.String("output-dir", "./output", "sample files output dir")
	prefix := flag.String("prefix", "sample-", "file name prefix")
	ext := flag.String("extension", "mpg", "file name extension")

	flag.Parse()

	err := os.MkdirAll(*outputDir, 0775)
	if err != nil {
		fmt.Println(err)
		return
	}

	sample, err := os.Create("sample-data.dat")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer sample.Close()

	for i := 0; i < *fileCount; i++ {
		filename := *prefix + uuid.NewV4().String() + "." + *ext
		f, err := os.Create(filepath.Join(*outputDir, filename))
		if err != nil {
			fmt.Println(err)
			return
		}
		f.Close()
		fmt.Fprintln(sample, filename)
	}

	fmt.Println("success to generate sample files : ", *outputDir)
	fmt.Println("success to make sample data : sample-data.dat")
}
