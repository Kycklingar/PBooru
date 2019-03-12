package DataManager

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
)

func checksum(file io.Reader) (string, string) {
	var b bytes.Buffer
	b.ReadFrom(file)

	sh := sha256.New()
	sh.Write(b.Bytes())

	md := md5.New()
	md.Write(b.Bytes())

	return fmt.Sprintf("%x", sh.Sum(nil)), fmt.Sprintf("%x", md.Sum(nil))
}
