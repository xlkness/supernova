package utils

import (
	"bytes"
	"crypto/rand"
	"math/big"
	"strings"
)

var str = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

// GetGlobalIDFromPodName 从k8s pod名里获取最后一个key
func GetGlobalIDFromPodName(podName string) string {
	//podName := os.Getenv("POD_NAME")
	if podName == "" {
		return randomString(7)
	}
	strs := strings.Split(podName, "-")
	return strs[len(strs)-1]
}

// GetGlobalIDFromRandomString 随机一个全局id
func GetGlobalIDFromRandomString(len int) string {
	return randomString(len)
}

// randomString 随机任意长度字符串，长度6大概5w次左右有概率重复
func randomString(len int) string {
	var container string
	b := bytes.NewBufferString(str)
	length := b.Len()
	bigInt := big.NewInt(int64(length))
	for i := 0; i < len; i++ {
		randomInt, _ := rand.Int(rand.Reader, bigInt)
		container += string(str[randomInt.Int64()])
	}
	return container
}
