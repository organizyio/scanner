package item

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMigrationLargeIntegerAndOptionalFields(t *testing.T) {
	const input = `{"path":"/unicode/Ա.jpg","size":18446744073709551615,"phash":18446744073709551615,"created_at":null,"exif":{"LensSerial":"sample","NumericExtra":18446744073709551615}}`
	var row FileRecord
	if err := json.Unmarshal([]byte(input), &row); err != nil {
		t.Fatal(err)
	}
	if row.Size != ^uint64(0) || row.Phash == nil || *row.Phash != ^uint64(0) || row.CreatedAt != nil {
		t.Fatalf("record %+v", row)
	}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "created_at") || !strings.Contains(string(b), "18446744073709551615") {
		t.Fatalf("encoding %s", b)
	}
	if string(row.Exif.Extra["NumericExtra"]) != "18446744073709551615" {
		t.Fatal("EXIF number rounded")
	}
}
