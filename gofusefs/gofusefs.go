package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"syscall"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

var readDir string
var useDirectio bool

type HelloFs struct {
	pathfs.FileSystem
}

func (me *HelloFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	fpath := path.Join(readDir, name)
	info, err := os.Stat(fpath)
	if err != nil {
		fmt.Printf("error os.stat(%v): %v\n", fpath, err)
		return nil, fuse.ENOENT
	}

	if info.IsDir() {
		return &fuse.Attr{
			Mode: fuse.S_IFDIR | 0755,
		}, fuse.OK
	}

	return &fuse.Attr{
		Mode:  fuse.S_IFREG | 0644,
		Size:  uint64(info.Size()),
		Mtime: uint64(info.ModTime().Unix()),
	}, fuse.OK
}

func (me *HelloFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	fpath := path.Join(readDir, name)
	files, err := ioutil.ReadDir(fpath)
	if err != nil {
		fmt.Printf("error ioutil.ReadDir(%v): %v\n", fpath, err)
		return nil, fuse.ENOENT
	}

	for _, f := range files {
		var mode uint32
		if f.IsDir() {
			mode = fuse.S_IFDIR
		} else {
			mode = fuse.S_IFREG
		}
		c = append(c, fuse.DirEntry{
			Name: f.Name(),
			Mode: mode,
		})
	}
	return c, fuse.OK
}

func (me *HelloFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	fpath := path.Join(readDir, name)
	f := new(helloFile)
	f.File = nodefs.NewDefaultFile()
	var err error
	f.file, err = os.OpenFile(fpath, os.O_RDONLY|syscall.O_DIRECT, 0666)
	if err != nil {
		fmt.Printf("error os.OpenFile(%v): %v\n", fpath, err)
		return nil, fuse.ENOENT
	}
	return f, fuse.OK
}

type helloFile struct {
	nodefs.File
	file *os.File
}

func (f *helloFile) GetAttr(out *fuse.Attr) fuse.Status {
	info, err := f.file.Stat()
	if err != nil {
		fmt.Printf("error os.stat(%v): %v\n", f.file.Name(), err)
		return fuse.ENOENT
	}
	out.Mode = fuse.S_IFREG | 0644
	out.Size = uint64(info.Size())
	return fuse.OK
}

func (f *helloFile) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	fmt.Printf("read: len = %d\n", len(buf))
	_, err := f.file.Seek(off, 0)
	if err != nil {
		fmt.Println(err)
		return nil, fuse.EACCES
	}

	const alignSize = 4096
	sz := int((len(buf)-1)/alignSize)*alignSize + alignSize
	newbuf := make([]byte, sz)
	// newbuf := directio.AlignedBlock(sz)
	n, err := f.file.Read(newbuf)
	if err != nil {
		fmt.Println(err)
		return nil, fuse.EACCES
	}
	return fuse.ReadResultData(newbuf[:n]), fuse.OK
	// return nil, fuse.ENOSYS
}

func (f *helloFile) Release() {
	f.file.Close()
}

func main() {
	mountPoint := flag.String("dir", "", "mount directory")
	readDirectory := flag.String("read-dir", "", "directory to read file")
	flag.Parse()

	readDir = *readDirectory

	nfs := pathfs.NewPathNodeFs(&HelloFs{FileSystem: pathfs.NewDefaultFileSystem()}, nil)
	server, _, err := nodefs.MountRoot(*mountPoint, nfs.Root(), nil)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	server.Serve()
}
