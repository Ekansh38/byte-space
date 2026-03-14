package utils

import (
	"github.com/spf13/afero"
)

func ReadLine(f afero.File) (string, error) {

	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := f.Read(buf)
		if err != nil {
			return string(line), err
		}
		if n == 0 {
			break
		}
		if buf[0] == '\n' {
			break
		}
		line = append(line, buf[0])
	}

	return string(line), nil
}
