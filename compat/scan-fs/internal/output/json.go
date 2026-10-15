package output

import (
	"encoding/json"
	"io"

	"github.com/meshory/scan-fs/pkg/item"
)

// JSONEncoder returns an encoder suitable for one JSON value per line (NDJSON).
func JSONEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}

// JSONLine writes a single JSON object and a trailing newline.
func JSONLine(enc *json.Encoder, it item.Item) error {
	return enc.Encode(it)
}
