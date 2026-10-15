package scan

type FileRecord struct {
	Path       string  `json:"path"`
	Name       string  `json:"name"`
	Directory  string  `json:"directory"`
	Type       string  `json:"type"` // "file"
	Extension  string  `json:"extension"`
	SizeBytes  int64   `json:"size_bytes"`
	SizeMB     float64 `json:"size_mb"`
	CreatedAt  string  `json:"created_at"`
	ModifiedAt string  `json:"modified_at"`
	AccessedAt string  `json:"accessed_at,omitempty"`
	ReadOnly   bool    `json:"read_only"`
	Hidden     bool    `json:"hidden"`
	SystemFile bool    `json:"system_file"`
	Sha256     string  `json:"sha256"`
	HashStatus string  `json:"hash_status"` // ok | skipped | error
}

type FolderRecord struct {
	Path              string           `json:"path"`
	Depth             int              `json:"depth"`
	FileCount         int              `json:"file_count"`
	SubfolderCount    int              `json:"subfolder_count"`
	TotalSizeBytes    int64            `json:"total_size_bytes"`
	Extensions        map[string]int64 `json:"extensions"`
	DominantExtension string           `json:"dominant_extension"`
	UniqueExtensions  int              `json:"unique_extensions"`
	ExtensionEntropy  float64          `json:"extension_entropy"`
	OldestModifiedAt  string           `json:"oldest_modified_at"`
	NewestModifiedAt  string           `json:"newest_modified_at"`
	IsProjectRoot     bool             `json:"is_project_root"`
	ProjectKind       string           `json:"project_kind"`
	MarkerFiles       map[string]bool  `json:"-"`
}

type ScanResult struct {
	Files   []FileRecord
	Folders map[string]*FolderRecord
}

type ScanOptions struct {
	MaxFileSizeMB     int64
	HashFiles         bool
	HashMaxFileSizeMB int64
	ProjectDetection  bool

	OnWarning    func(path string, err error)
	Pause        <-chan bool
	OnCheckpoint func(string)

	Progress *Progress
	Runtime  *RuntimeOptions // for live changes mid-scan (advanced)
}
