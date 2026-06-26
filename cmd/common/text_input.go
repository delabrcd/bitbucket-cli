package common

import (
	"io"
	"os"

	"github.com/gildas/go-errors"
)

// ReadFileOrStdin reads the entire content of the file at path.
// A path of "-" reads from standard input instead.
func ReadFileOrStdin(path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, errors.RuntimeError.Wrap(err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.RuntimeError.Wrap(err)
	}
	return data, nil
}
