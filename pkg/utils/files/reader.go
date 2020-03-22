package files

import (
	"io/ioutil"
	"os"
)

func ReadFile(path string) (content []byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	content, err = ioutil.ReadAll(f)
	return
}
