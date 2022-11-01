package utils

import (
	"crypto/md5"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
)

var OpenFileFunc func(string) (fs.File, error) = func(path string) (fs.File, error) {
	fd, err := os.Open(path)
	return fd, err
}

type MyMD5 struct {
	md5Hasher hash.Hash
	buf       []byte
}

func (md5 *MyMD5) Size() int { return md5.md5Hasher.Size() }

func (md5 *MyMD5) BlockSize() int { return md5.md5Hasher.BlockSize() }

func (md5 *MyMD5) Write(b []byte) (int, error) {
	md5.buf = b[:len(b)]
	return md5.md5Hasher.Write(b)
}

func (md5 *MyMD5) Sum() []byte {
	return md5.md5Hasher.Sum(nil)
}

func (md5 *MyMD5) Reset() {
	md5.buf = md5.buf[:0]
	md5.md5Hasher.Reset()
}

func CheckMD5(file, oldMd5 string) (newMd5 string, same bool, err error) {
	var md5Checker = &MyMD5{md5Hasher: md5.New()}
	fd, err := OpenFileFunc(file)
	if err != nil {
		return oldMd5, false, fmt.Errorf("open file %v error:%v", file, err)
	}
	defer fd.Close()
	if _, err := io.Copy(md5Checker, fd); err != nil {
		return oldMd5, false, fmt.Errorf("io.Copy file %v error:%v", file, err)
	}

	newMd5 = string(md5Checker.Sum())

	if oldMd5 == "" {
		return newMd5, false, nil
	}

	return newMd5, oldMd5 == newMd5, nil
}
