package jlog

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

type RotatingDayMaxFileHandler struct {
	baseName string
	outPath  string
	fd       *os.File

	rotateInfo struct {
		maxBytes       int       // 单个日志文件最大长度
		maxBackupCount int       // 最大归档文件数量
		day            time.Time // 记录跨天
	}
}

// NewRotatingDayMaxFileHandler 每天00点滚动，当超过大小也滚动
func NewRotatingDayMaxFileHandler(outPath, baseName string, maxBytes int, backupCount int) (*RotatingDayMaxFileHandler, error) {
	if outPath == "" {
		outPath = "log/"
	}

	h := new(RotatingDayMaxFileHandler)
	h.baseName = baseName
	h.outPath = outPath
	h.rotateInfo.day = time.Now()
	h.rotateInfo.maxBytes = maxBytes
	h.rotateInfo.maxBackupCount = backupCount
	fd, err := openFile(h.outPath, h.baseName, false)
	if err != nil {
		panic(err)
	} else {
		h.fd = fd
	}
	go h.rotateHandler()
	return h, nil
}

func (h *RotatingDayMaxFileHandler) Write(p []byte) (n int, err error) {
	return h.fd.Write(p)
}

func (h *RotatingDayMaxFileHandler) Close() error {
	if h.fd != nil {
		return h.fd.Close()
	}
	return nil
}

func (h *RotatingDayMaxFileHandler) rotateHandler() {
	for {
		time.Sleep(time.Second)
		h.tryRotate()
	}
}

func (h *RotatingDayMaxFileHandler) tryRotate() {
	// 校验时间是否触发归档
	now := time.Now()
	if now.Day() != h.rotateInfo.day.Day() {
		h.rotateDay()
		h.rotateInfo.day = now
		return
	}

	// 校验文件大小是否触发归档
	size, err := calcFileSize(h.fd)
	if err != nil {
		outErrorLog("stat log file(%v) error:%v", h.baseName, err)
		return
	}

	if h.rotateInfo.maxBytes > 0 && h.rotateInfo.maxBytes <= size {
		h.rotateSize()
		return
	}
}

// rotateAt 到点触发归档
func (h *RotatingDayMaxFileHandler) rotateDay() {
	// 创建新的一天的日志文件
	newFd, err := openFile(h.outPath, h.baseName, false)
	if err != nil {
		outErrorLog("new day rotate file, but open new file(%v) error:%v", h.baseName, err)
		return
	}

	// 用新的一天的日志文件描述符接管当前使用的
	oldFd := h.fd
	h.fd = newFd
	oldFd.Close()
}

// rotateSize 文件过大触发归档
func (h *RotatingDayMaxFileHandler) rotateSize() {
	// 锁定文件，使触发归档的别的进程也锁住
	lockFile(h.fd)
	// 重新打开文件判断大小，防止文件被别的归档进程改名
	curSize := calcFileNameSize(h.fd.Name())
	if curSize < h.rotateInfo.maxBytes {
		// 别的进程归档过了
		unlockFile(h.fd)
		return
	}

	// 滚动copy归档的文件，那么归档的1号文件空出来了
	baseFileName := h.fd.Name()
	for i := h.rotateInfo.maxBackupCount; i > 0; i-- {
		sfn := fmt.Sprintf("%s.%d", baseFileName, i)
		dfn := fmt.Sprintf("%s.%d", baseFileName, i+1)
		os.Rename(sfn, dfn)
	}

	// 将当前文件内容拷贝到归档1号文件
	dfn := fmt.Sprintf("%s.1", baseFileName)
	os.Rename(baseFileName, dfn)

	// 重新创建当前日志文件得到新的描述符
	newFd, err := os.OpenFile(baseFileName, os.O_CREATE|os.O_APPEND|os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		outErrorLog("rotate log file size, open file(%v) error:%v", baseFileName, err)
		unlockFile(h.fd)
	} else {
		oldFd := h.fd
		h.fd = newFd
		unlockFile(h.fd)
		oldFd.Close()
	}
}

func calcFileSize(fd *os.File) (int, error) {
	st, err := fd.Stat()
	return int(st.Size()), err
}

func calcFileNameSize(fileName string) int {
	fd, err := os.OpenFile(fileName, os.O_RDWR|os.O_APPEND, 0777)
	if err != nil {
		return 0
	}
	defer fd.Close()
	st, err := fd.Stat()
	if err != nil {
		return 0
	}
	return int(st.Size())
}

var fileTimeStampFormat = "20060102"

func openFile(path, baseName string, isTrunc bool) (*os.File, error) {
	os.MkdirAll(path, 0777)
	timeNow := time.Now()
	t := timeNow.Format(fileTimeStampFormat)
	fullFileName := baseName + "-" + t + ".log"

	if path != "" {
		if path[len(path)-1] != '/' {
			path += "/"
		}
	}

	fullPathFileName := path + fullFileName

	var fd *os.File
	var err error

	flag := os.O_CREATE | os.O_APPEND | os.O_RDWR
	if isTrunc {
		flag |= os.O_TRUNC
	}

	fd, err = os.OpenFile(fullPathFileName, flag, 0777)
	if err != nil {
		return fd, fmt.Errorf("open log file(%v) error:%v", fullPathFileName, err)
	}
	return fd, nil
}

func lockFile(fd *os.File) {
	err := syscall.Flock(int(fd.Fd()), syscall.LOCK_EX)
	if err != nil {
		outErrorLog("lock file(%v) error:%v", fd.Name(), err)
	}
}

func unlockFile(fd *os.File) {
	err := syscall.Flock(int(fd.Fd()), syscall.LOCK_UN)
	if err != nil {
		outErrorLog("unlock file(%v) error:%v", fd.Name(), err)
	}
}

func outErrorLog(format string, values ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
}
