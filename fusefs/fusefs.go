package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/net/context"
)

var inode uint64
var readDir string
var useDirectio bool
var useNoRead bool
var contentSize int64

func run(dir string, readahead int) error {
	inode = 0
	c, err := fuse.Mount(
		dir,
		fuse.FSName("cache-fs"),
		fuse.Subtype("cache-fs"),
		fuse.LocalVolume(),
		fuse.VolumeName("cache filesystem"),
		fuse.MaxReadahead(uint32(readahead)),
		fuse.AsyncRead(),
		fuse.AllowOther(),
	)
	if err != nil {
		return err
	}
	defer c.Close()

	srv := fs.New(c, nil)
	filesys := &WebFS{mountdir: dir}
	if err := srv.Serve(filesys); err != nil {
		return err
	}

	<-c.Ready
	return c.MountError
}

// WebFS :
type WebFS struct {
	mountdir string
}

var _ fs.FS = (*WebFS)(nil)

// Root :
func (f *WebFS) Root() (fs.Node, error) {
	return &Dir{name: f.mountdir, parent: nil, isHost: false}, nil
}

// GenerateInode :
func (f *WebFS) GenerateInode(parentInode uint64, name string) uint64 {
	inode++
	return inode
}

// Dir :
type Dir struct {
	name   string
	parent *Dir
	isHost bool
}

var _ fs.Node = (*Dir)(nil)

// Attr :
func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0555
	return nil
}

var _ fs.NodeStringLookuper = (*Dir)(nil)

func (d *Dir) parentDirs() (dirs string) {
	for itr := d; itr != nil; itr = itr.parent {
		if itr.parent != nil {
			dirs = itr.name + "/" + dirs
		}
	}
	return
}

// Lookup :
func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	fmt.Printf("Lookup %v\n", name)
	fpath := path.Join(readDir, d.parentDirs(), name)
	info, err := os.Stat(fpath)
	if err != nil {
		fmt.Printf("error os.stat(%v): %v\n", fpath, err)
		return nil, fuse.Errno(syscall.ENOENT)
	}
	if info.IsDir() {
		return &Dir{name: name, parent: d, isHost: false}, nil
	}
	return &File{name: name, parent: d}, nil
}

// File : implemented fs.Node
type File struct {
	name   string
	parent *Dir
}

var _ fs.Node = (*File)(nil)
var _ fs.NodeOpener = (*File)(nil)

// Attr :
func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	fmt.Printf("File Attr %v, inode:%v\n", f.name, a.Inode)
	fpath := path.Join(readDir, f.parent.parentDirs(), f.name)
	info, err := os.Stat(fpath)
	if err != nil {
		fmt.Printf("error os.stat(%v): %v\n", fpath, err)
		return fuse.Errno(syscall.ENOENT)
	}

	a.Mode = 0644
	a.Mtime = info.ModTime()
	a.Size = uint64(info.Size())
	return nil
}

// Open :
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	//fmt.Printf("Open %v\n", f.name)
	if useNoRead {
		return &FileHandle{file: f}, nil
	}
	fpath := path.Join(readDir, f.parent.parentDirs(), f.name)

	flag := os.O_RDONLY
	if useDirectio {
		flag |= syscall.O_DIRECT
	}
	fp, err := os.OpenFile(fpath, flag, 0666)
	if err != nil {
		return nil, err
	}
	fi, err := fp.Stat()
	if err != nil {
		return nil, err
	}

	return &FileHandle{file: f, fp: fp, fi: fi, sid: uuid.NewV4().String()}, nil
}

// FileHandle : implemented fs.Handle
type FileHandle struct {
	file *File
	fp   *os.File
	fi   os.FileInfo
	sid  string
}

var _ fs.Handle = (*FileHandle)(nil)
var _ fs.HandleReader = (*FileHandle)(nil)
var _ fs.HandleReleaser = (*FileHandle)(nil)

var requestID = 0

func (h *FileHandle) readfile(filepath string, offset int64, len int) ([]byte, error) {
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

		o, err := h.fp.Seek(beg, 0)
		if err != nil {
			return nil, err
		}
		if o != beg {
			return nil, fmt.Errorf("seek result(%v) != offset(%v)", o, beg)
		}

		sz := int((len+alignSize)/alignSize) * alignSize
		if sz == int(len+int(alignSize)) {
			sz = int(len)
		}
		buf = make([]byte, sz)
		//buf = directio.AlignedBlock(sz)
		readed, err := h.fp.Read(buf)
		if err != nil {
			return nil, err
		}
		newlen := len
		if readed < newlen {
			newlen = readed
		}
		buf = buf[:newlen]
	} else {
		buf = make([]byte, len)
		o, err := h.fp.Seek(offset, 0)
		if err != nil {
			return nil, err
		}
		if o != offset {
			return nil, fmt.Errorf("seek result(%v) != offset(%v)", o, offset)
		}

		if _, err := h.fp.Read(buf); err != nil {
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

// Read :
func (h *FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	if useNoRead {
		sz := int64(req.Size)
		if req.Offset + int64(req.Size) > contentSize {
			sz = contentSize - req.Offset
		}
		resp.Data = make([]byte, sz)
		return nil
	}
	fmt.Printf("Read %v (offset:%v, len:%v, directio:%v)\n", h.file.name, req.Offset, req.Size, useDirectio)
	fpath := path.Join(readDir, h.file.parent.parentDirs(), h.file.name)
	bytes, err := h.readfile(fpath, req.Offset, req.Size)
	if err != nil {
		fmt.Printf("readfile error: %v\n", err)
		return fuse.Errno(syscall.EACCES)
	}
	resp.Data = bytes
	return nil
}

// Release :
func (h *FileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	//fmt.Printf("Release %v\n", h.file.name)
	if useNoRead {
		return nil
	}
	h.fp.Close()
	return nil
}

func main() {
	mountPoint := flag.String("dir", "", "mount directory")
	readDirectory := flag.String("read-dir", "", "directory to read file")
	readahead := flag.Int("read-ahead", 256, "fuse mount option : max read ahead bytes")
	directio := flag.Bool("directio", false, "use direct i/o")
	noread := flag.Bool("no-read", false, "no read. return fake bytes")
	fsize := flag.Int64("content-size", 5000000, "content file size")
	flag.Parse()

	readDir = *readDirectory
	useDirectio = *directio
	useNoRead = *noread
	contentSize = *fsize

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func() {
		for sig := range c {
			fmt.Printf("%v\n", sig)
			if err := fuse.Unmount(*mountPoint); err != nil {
				fmt.Printf("%v\n", err)
			}
			os.Exit(1)
		}
	}()

	if err := run(*mountPoint, *readahead); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
