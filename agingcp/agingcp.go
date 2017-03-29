package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/castisdev/cdn/hutil"
)

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

var importAddr string
var host string
var purgeAddr string

func purge(file string) {
	url := "http://" + path.Join(purgeAddr, "api/caches", host, file)

	cl := hutil.NewHTTPClient(0)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Fatalf("failed to purge %v, %v\n", url, err)
	}
	res, err := cl.Do(req)
	if err != nil {
		log.Fatalf("failed to purge %v, %v\n", url, err)
	}
	defer res.Body.Close()
	if res.StatusCode != 204 {
		log.Fatalf("failed to purge %v, error response %v\n", url, res)
	}
	log.Printf("sucess to purge %v\n", url)
}
func importFile(src, dst, rangeHeader string) {
	url := "http://" + path.Join(importAddr, path.Base(dst))
	log.Printf("import %v Range:%v ...\n", url, rangeHeader)

	cl := hutil.NewHTTPClient(0)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("error!! %v\n", err)
		return
	}
	req.Host = host
	req.Header.Set("Connection", "Close")
	req.Header.Set("Range", rangeHeader)

	res, err := cl.Do(req)
	if err != nil {
		fmt.Printf("error!! %v\n", err)
	}
	defer res.Body.Close()
	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	if res.StatusCode != 206 {
		fmt.Printf("response status code is not 206, %v\n", res.Status)
		return
	}
	return
}

func link(mode, src, dst string, importSize int64) {
	if importAddr != "" {
		importFile(src, dst, "bytes=0-"+strconv.FormatInt(importSize, 10))

		var ra string
		switch {
		case strings.Contains(src, "HD_2001"):
			ra = "bytes=3831439224-3841439223"
		case strings.Contains(src, "HD_2021"):
			ra = "bytes=4246566522-4256566521"
		case strings.Contains(src, "HD_2041"):
			ra = "bytes=1446684742-1456684741"
		case strings.Contains(src, "SD_2001"):
			ra = "bytes=1685212170-1695212169"
		}
		importFile(src, dst, ra)
	}

	if mode == "copy" {
		if err := os.Link(src, dst); err == nil {
			return
		} else {
			if err := copyFileContents(src, dst); err != nil {
				log.Fatal(err)
			}
		}
	} else {
		if err := os.Symlink(src, dst); err != nil {
			log.Fatal(err)
		}
	}
}

func main() {
	mode := flag.String("mode", "copy", "copy / link, source file: org_HD_2001.mpg / org_HD_2021.mpg / org_HD2041.mpg / org_SD_2001.mpg")
	srcDir := flag.String("src", ".", "source directory")
	dstDir := flag.String("dst", ".", "destination directory")
	importA := flag.String("import-addr", "", "cache url to import")
	importR := flag.Int("import-rate", 100, "rate of file count to import, value should be multiple of 5.")
	ho := flag.String("host", "eve", "host")
	purgeA := flag.String("purge-addr", "", "cache api address to purge")
	purgeR := flag.Int("purge-rate", 100, "rate of file count to purge, value should be multiple of 5.")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	importAddr = *importA
	importRate := *importR
	host = *ho
	purgeRate := *purgeR
	purgeAddr = *purgeA

	if importRate%5 != 0 {
		log.Fatalf("import-rate should be multiple of 5 (%v)", importRate)
	}
	if purgeRate%5 != 0 {
		log.Fatalf("purge-rate should be multiple of 5 (%v)", purgeRate)
	}
	importMode := importAddr != ""
	purgeMode := purgeAddr != ""

	importCntHD := int(importRate * 20 / 100)
	purgeCntHD := int(purgeRate * 20 / 100)
	for i := 1; i <= 20; i++ {
		{
			src := path.Join(*srcDir, "org_HD_2001.mpg")

			nstr := fmt.Sprintf("%03d", i)
			dst := path.Join(*dstDir, "HD_2"+nstr+".mpg")
			nstr2 := fmt.Sprintf("%03d", i+100)
			dst2 := path.Join(*dstDir, "HD_2"+nstr2+".mpg")
			nstr3 := fmt.Sprintf("%03d", i+200)
			dst3 := path.Join(*dstDir, "HD_2"+nstr3+".mpg")

			if purgeMode {
				if purgeCntHD >= i {
					purge(dst)
					purge(dst2)
					purge(dst3)
				}
			} else {
				if !importMode || importCntHD >= i {
					link(*mode, src, dst, 1000000000)
					link(*mode, src, dst2, 1000000000)
					link(*mode, src, dst3, 1000000000)
				}
			}
		}
		{
			nstr := fmt.Sprintf("%03d", i+20)
			src := path.Join(*srcDir, "org_HD_2021.mpg")
			dst := path.Join(*dstDir, "HD_2"+nstr+".mpg")
			nstr2 := fmt.Sprintf("%03d", i+120)
			dst2 := path.Join(*dstDir, "HD_2"+nstr2+".mpg")
			nstr3 := fmt.Sprintf("%03d", i+220)
			dst3 := path.Join(*dstDir, "HD_2"+nstr3+".mpg")

			if purgeMode {
				if purgeCntHD >= i {
					purge(dst)
					purge(dst2)
					purge(dst3)
				}
			} else {
				if !importMode || importCntHD >= i {
					link(*mode, src, dst, 1000000000)
					link(*mode, src, dst2, 1000000000)
					link(*mode, src, dst3, 1000000000)
				}
			}
		}
		{
			nstr := fmt.Sprintf("%03d", i+40)
			src := path.Join(*srcDir, "org_HD_2041.mpg")
			dst := path.Join(*dstDir, "HD_2"+nstr+".mpg")
			nstr2 := fmt.Sprintf("%03d", i+140)
			dst2 := path.Join(*dstDir, "HD_2"+nstr2+".mpg")
			nstr3 := fmt.Sprintf("%03d", i+240)
			dst3 := path.Join(*dstDir, "HD_2"+nstr3+".mpg")

			if purgeMode {
				if purgeCntHD >= i {
					purge(dst)
					purge(dst2)
					purge(dst3)
				}
			} else {
				if !importMode || importCntHD >= i {
					link(*mode, src, dst, 1400000000)
					link(*mode, src, dst2, 1400000000)
					link(*mode, src, dst3, 1400000000)
				}
			}
		}
	}

	importCntSD := int(importRate * 600 / 100)
	purgeCntSD := int(purgeRate * 600 / 100)
	for i := 1; i <= 600; i++ {
		{
			nstr := fmt.Sprintf("%03d", i)
			src := path.Join(*srcDir, "org_SD_2001.mpg")
			dst := path.Join(*dstDir, "SD_2"+nstr+".mpg")

			if purgeMode {
				if purgeCntSD >= i {
					purge(dst)
				}
			} else {
				if !importMode || importCntSD >= i {
					link(*mode, src, dst, 450000000)
				}
			}
		}
	}
}
