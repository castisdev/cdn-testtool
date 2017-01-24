#define FUSE_USE_VERSION 27

#include <fuse.h>
#include <stdio.h>
#include <string.h>
#include <errno.h>
#include <fcntl.h>

#include <iostream>
#include <atomic>
#include <map>
#include <mutex>

#include <boost/filesystem.hpp>
#include <boost/filesystem/fstream.hpp>

#include "fusecpp.h"

using namespace fuse_cpp;
namespace fs = boost::filesystem;

struct FuseFS {
    std::atomic<uint64_t> fh;
	fs::path source_dir_;
	std::map<uint64_t, std::shared_ptr<fs::ifstream>> fdMap_;
	std::mutex mutex_;
};

FuseFS fuseFS;

static const char *hello_path = "/hello";

static int hello_getattr(const char *path, struct stat *stbuf)
{
	auto fullPath = fuseFS.source_dir_ / path;

	if (fs::exists(fullPath) == false)
		return -ENOENT;
	
	memset(stbuf, 0, sizeof(struct stat));

	if (fs::is_directory(fullPath)) {
		stbuf->st_mode = S_IFDIR | 0755;
		stbuf->st_nlink = 2;
	} else {
		stbuf->st_mode = S_IFREG | 0444;
		stbuf->st_nlink = 1;
		stbuf->st_size = fs::file_size(fullPath);
	}

	return 0;
}

static int hello_readdir(const char *path, void *buf, fuse_fill_dir_t filler,
			 off_t offset, struct fuse_file_info *fi)
{
	(void) offset;
	(void) fi;

	if (strcmp(path, "/") != 0)
		return -ENOENT;

	filler(buf, ".", NULL, 0);
	filler(buf, "..", NULL, 0);
	filler(buf, hello_path + 1, NULL, 0);

	return 0;
}

uint64_t fh = 1;

static int hello_open(const char *path, struct fuse_file_info *fi)
{
	auto fullPath = fuseFS.source_dir_ / path;

	if ((fi->flags & 3) != O_RDONLY)
		return -EACCES;

	auto ifs = std::make_shared<fs::ifstream>(fullPath);

	if (!*ifs)
		return -EACCES;

    fi->fh = ++fuseFS.fh;

	std::lock_guard<std::mutex> lock(fuseFS.mutex_);
	fuseFS.fdMap_.emplace(fi->fh, ifs);

	return 0;
}

static int hello_read(const char *path, char *buf, size_t size, off_t offset,
		      struct fuse_file_info *fi)
{
	auto fullPath = fuseFS.source_dir_ / path;
	std::shared_ptr<fs::ifstream> ifs;

	{
		std::lock_guard<std::mutex> lock(fuseFS.mutex_);
		auto pos = fuseFS.fdMap_.find(fi->fh);
		if (pos == fuseFS.fdMap_.end())
			return -ENOENT;
		ifs = pos->second;
	}
	
	if (!ifs)
		return -ENOENT;

	ifs->seekg(offset);
	return ifs->readsome(buf, size);
}

static int hello_release(const char *path, struct fuse_file_info *fi)
{
	auto fullPath = fuseFS.source_dir_ / path;
	std::lock_guard<std::mutex> lock(fuseFS.mutex_);
	fuseFS.fdMap_.erase(fi->fh);
    return 0;
}

int main(int argc, char *argv[])
{
    if (argc < 3) {
        printf("%s <mount dir> <src dir>\n", argv[0]);
        exit(-1);
    }
	FuseDispatcher *dispatcher;
	dispatcher = new FuseDispatcher();
	dispatcher->set_getattr(&hello_getattr);
	dispatcher->set_open(&hello_open);
	dispatcher->set_read(&hello_read);
	dispatcher->set_readdir(&hello_readdir);
	dispatcher->set_release(&hello_release);
	fuseFS.source_dir_ = argv[2];
	return fuse_main(2, argv, (dispatcher->get_fuseOps()), NULL);
}
