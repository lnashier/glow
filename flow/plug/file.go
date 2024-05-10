package plug

import (
	"bufio"
	"io"
	"os"
)

// StepReadFile reads len bytes from file and calls provided emit function.
// If exact flag is set, it reads exactly len bytes from file.
// An error is returned if fewer than len bytes were read.
func StepReadFile(name string, len int, exact bool, emit func([]byte) error) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReader(f)

	var rf func(r io.Reader) (int, []byte, error)
	switch {
	case exact:
		rf = func(r io.Reader) (int, []byte, error) {
			buf := make([]byte, len)
			n, err := io.ReadFull(r, buf)
			return n, buf[:n], err
		}
	default:
		rf = func(r io.Reader) (int, []byte, error) {
			buf := make([]byte, len)
			n, err := r.Read(buf)
			return n, buf, err
		}
	}

	for {
		n, buf, err := rf(r)
		if n > 0 {
			if err = emit(buf); err != nil {
				return err
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return nil
}

// ReadFile read the entire file and calls provided emit function.
func ReadFile(name string, emit func(data any)) error {
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	emit(string(data))
	return nil
}
