package files

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

func Sha256Checksum(file string) (checksum string, err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()

	h := sha256.New()
	if _, err2 := io.Copy(h, f); err2 != nil {
		err = err2
		return
	}

	checksum = fmt.Sprintf("%x", h.Sum(nil))
	return
}

func Sha1ChecksumFile(file string) (checksum string, err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	return Sha1Checksum(f)
}

func Sha1Checksum(reader io.Reader) (checksum string, err error) {
	h := sha1.New()
	if _, err := io.Copy(h, reader); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func Sha1ChecksumString(str string) string {
	h := sha1.New()
	h.Write([]byte(str))
	return fmt.Sprintf("%x", h.Sum(nil))
}
