package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// ArchiveInfo holds metadata about an existing archive file.
type ArchiveInfo struct {
	Path      string // Full filesystem path
	Filename  string // Base filename
	Size      int64  // File size in bytes
	Timestamp string // From manifest, or file mod time
	MudName   string // From manifest
	Objects   int    // From manifest
}

// ListArchives scans an archive directory for .tar.gz files and returns info
// about each, sorted newest-first.
func ListArchives(archiveDir string) ([]ArchiveInfo, error) {
	pattern := filepath.Join(archiveDir, "*.tar.gz")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("archive: glob %s: %w", pattern, err)
	}

	var archives []ArchiveInfo
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		ai := ArchiveInfo{
			Path:      path,
			Filename:  filepath.Base(path),
			Size:      info.Size(),
			Timestamp: info.ModTime().Format("2006-01-02 15:04:05"),
		}

		// Try to read manifest for richer metadata
		if m, err := readManifest(path); err == nil {
			ai.Timestamp = m.Timestamp
			ai.MudName = m.MudName
			ai.Objects = m.Objects
		}

		archives = append(archives, ai)
	}

	// Sort newest-first by timestamp string (RFC3339 sorts lexically)
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].Timestamp > archives[j].Timestamp
	})

	return archives, nil
}

// readManifest opens a .tar.gz file and extracts the manifest.json entry.
func readManifest(archivePath string) (*Manifest, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == "manifest.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			var m Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, err
			}
			return &m, nil
		}
	}
	return nil, fmt.Errorf("manifest.json not found in archive")
}
