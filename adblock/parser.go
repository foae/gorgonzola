package adblock

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// PeekFile peeks into a file and reads a `peekSize` of bytes, returning them.
func PeekFile(fullFilePath string, peekSize int) ([]byte, error) {
	fh, err := os.Open(fullFilePath)
	if err != nil {
		return nil, fmt.Errorf("repo: could not open file (%v): %v", fullFilePath, err)
	}

	if peekSize <= 0 || peekSize >= 32*1024 {
		peekSize = 64
	}

	buf := make([]byte, peekSize)
	_, err = fh.Read(buf)
	_ = fh.Close()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("repo: could not read file (%v): %v", fullFilePath, err)
	}

	return buf, nil
}

func IsFileAdBlockPlusFormat(peek []byte) bool {
	return bytes.Contains(peek, []byte("[Adblock Plus"))
}
