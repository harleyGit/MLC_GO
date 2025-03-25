package hg_uuid

import (
	"crypto/rand"
	"fmt"
)

const uuidLen = 16


func GenerateUUID() (string, error) {
	buf, err := GenerateRandomBytes(uuidLen)
	if err != nil {
		return "", err
	}
	return FormatUUID(buf)
}

func GenerateRandomBytes(size int) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %v", err)
	}
	return buf, nil
}

func FormatUUID(buf []byte) (string, error) {
	if buflen := len(buf); buflen != uuidLen {
		return "", fmt.Errorf("wrong length byte slice (%d)", buflen)
	}

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16]), nil
}