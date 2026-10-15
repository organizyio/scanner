package output

import (
	"io"
	"strconv"

	"github.com/meshory/scan-fs/pkg/item"
	"github.com/olekukonko/tablewriter"
)

// Table buffers rows and renders a table when Flush is called.
type Table struct {
	tw *tablewriter.Table
}

// NewTable builds a table writer for scan results.
func NewTable(w io.Writer) *Table {
	tw := tablewriter.NewWriter(w)
	tw.SetHeader([]string{"Path", "Directory", "Name", "Ext", "Size", "xxhash", "sha256", "mime"})
	tw.SetAutoWrapText(false)
	return &Table{tw: tw}
}

// Add appends one row from an item.
func (t *Table) Add(it item.Item) {
	xx := ""
	if it.Xxhash != nil {
		xx = *it.Xxhash
	}
	sh := ""
	if it.Sha256 != nil {
		sh = *it.Sha256
	}
	mime := ""
	if it.Mime != nil {
		mime = *it.Mime
	}
	t.tw.Append([]string{
		it.Path,
		it.Directory,
		it.Name,
		it.Extension,
		strconv.FormatUint(it.Size, 10),
		xx,
		sh,
		mime,
	})
}

// Flush renders the table.
func (t *Table) Flush() {
	t.tw.Render()
}
