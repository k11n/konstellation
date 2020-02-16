package objects

import (
	"bytes"
	"encoding/gob"
)

func Clone(dst, src interface{}) {
	buff := new(bytes.Buffer)
	enc := gob.NewEncoder(buff)
	dec := gob.NewDecoder(buff)
	enc.Encode(src)
	dec.Decode(dst)
}
