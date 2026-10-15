package output

import (
	"fmt"
	"io"

	"github.com/meshory/scan-fs/pkg/item"
)

// Plain writes one filesystem path per line.
func Plain(w io.Writer, it item.Item) error {
	_, err := fmt.Fprintln(w, it.Path)
	return err
}
