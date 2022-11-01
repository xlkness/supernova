package jlog

import (
	"os"
	"path"
	"sync"
)

//FileHandler writes log to a file.
type FileHandler struct {
	fileName string
	flag     int
	fd       *os.File
	fdLock   *sync.RWMutex
}

func NewFileHandler(fileName string, flag int) (*FileHandler, error) {
	dir := path.Dir(fileName)
	os.Mkdir(dir, 0777)

	f, err := os.OpenFile(fileName, flag, 0666)
	if err != nil {
		return nil, err
	}

	h := new(FileHandler)

	h.fileName = fileName
	h.flag = flag
	h.fd = f
	h.fdLock = &sync.RWMutex{}

	return h, nil
}

//func (h *FileHandler) Rotate() {
//	fd, err := os.OpenFile(h.fileName, h.flag, 0666)
//	if err != nil {
//		h.fd.Write([]byte(fmt.Sprintf("interval check log file %v open error:%v", h.fileName, err)))
//		return
//	}
//
//	h.fdLock.Lock()
//	oldFd := h.fd
//	h.fd = fd
//	oldFd.Close()
//	h.fdLock.Unlock()
//}

func (h *FileHandler) Write(b []byte) (n int, err error) {
	h.fdLock.RLock()
	defer h.fdLock.RUnlock()
	return h.fd.Write(b)
}

func (h *FileHandler) Close() error {
	return h.fd.Close()
}
