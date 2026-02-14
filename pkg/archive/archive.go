package archive

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Manifest describes the contents of an archive.
type Manifest struct {
	Version   int                  `json:"version"`
	Server    string               `json:"server"`
	Timestamp string               `json:"timestamp"`
	MudName   string               `json:"mud_name"`
	Objects   int                  `json:"objects"`
	Files     map[string]FileEntry `json:"files"`
}

// FileEntry describes a single file within the archive.
type FileEntry struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Type   string `json:"type"` // "bolt", "sql", "dict", "text", "conf"
}

// ArchiveParams holds all inputs needed to create an archive.
type ArchiveParams struct {
	BoltSnapshotFunc  func(destPath string) error // Caller provides bolt snapshot closure
	SQLPath           string                      // Path to SQLite database (empty = skip)
	SQLCheckpointFunc func() error                // Checkpoint WAL before copy (nil = skip)
	DictDir           string                      // Path to dictionary directory (empty = skip)
	TextDir           string                      // Path to text files directory (empty = skip)
	ConfPath          string                      // Path to game config file (empty = skip)
	AliasConfs        []string                    // Paths to alias config files
	ArchiveDir        string                      // Output directory for the archive
	MudName           string                      // MUD name for manifest
	ObjectCount       int                         // Number of objects for manifest
}

// CreateArchive creates a .tar.gz archive of all game data and returns the archive path.
func CreateArchive(params ArchiveParams) (string, error) {
	if err := os.MkdirAll(params.ArchiveDir, 0755); err != nil {
		return "", fmt.Errorf("archive: create dir %s: %w", params.ArchiveDir, err)
	}

	filename := fmt.Sprintf("archive-%s.tar.gz", time.Now().Format("20060102-150405"))
	archivePath := filepath.Join(params.ArchiveDir, filename)

	// Create temp dir for staging
	tmpDir, err := os.MkdirTemp("", "mush-archive-*")
	if err != nil {
		return "", fmt.Errorf("archive: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	manifest := Manifest{
		Version:   1,
		Server:    "GoTinyMUSH",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		MudName:   params.MudName,
		Objects:   params.ObjectCount,
		Files:     make(map[string]FileEntry),
	}

	// Stage bolt snapshot
	var boltStaged string
	if params.BoltSnapshotFunc != nil {
		boltStaged = filepath.Join(tmpDir, "game.bolt")
		if err := params.BoltSnapshotFunc(boltStaged); err != nil {
			return "", fmt.Errorf("archive: bolt snapshot: %w", err)
		}
	}

	// Stage SQL copy
	var sqlStaged string
	if params.SQLPath != "" {
		if params.SQLCheckpointFunc != nil {
			if err := params.SQLCheckpointFunc(); err != nil {
				return "", fmt.Errorf("archive: sql checkpoint: %w", err)
			}
		}
		sqlStaged = filepath.Join(tmpDir, "game.sqldb")
		if err := copyFile(params.SQLPath, sqlStaged); err != nil {
			return "", fmt.Errorf("archive: copy sql: %w", err)
		}
	}

	// Open output tar.gz
	outFile, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("archive: create %s: %w", archivePath, err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Add bolt snapshot
	if boltStaged != "" {
		entry, err := addFileToTar(tw, boltStaged, "data/game.bolt")
		if err != nil {
			return "", err
		}
		entry.Type = "bolt"
		manifest.Files["data/game.bolt"] = entry
	}

	// Add SQL database
	if sqlStaged != "" {
		entry, err := addFileToTar(tw, sqlStaged, "data/game.sqldb")
		if err != nil {
			return "", err
		}
		entry.Type = "sql"
		manifest.Files["data/game.sqldb"] = entry
	}

	// Add dictionary directory
	if params.DictDir != "" {
		if info, err := os.Stat(params.DictDir); err == nil && info.IsDir() {
			entries, err := addDirToTar(tw, params.DictDir, "data/dict")
			if err != nil {
				return "", err
			}
			for k, v := range entries {
				v.Type = "dict"
				manifest.Files[k] = v
			}
		}
	}

	// Add text directory
	if params.TextDir != "" {
		if info, err := os.Stat(params.TextDir); err == nil && info.IsDir() {
			entries, err := addDirToTar(tw, params.TextDir, "text")
			if err != nil {
				return "", err
			}
			for k, v := range entries {
				v.Type = "text"
				manifest.Files[k] = v
			}
		}
	}

	// Add config file
	if params.ConfPath != "" {
		if _, err := os.Stat(params.ConfPath); err == nil {
			archName := "conf/" + filepath.Base(params.ConfPath)
			entry, err := addFileToTar(tw, params.ConfPath, archName)
			if err != nil {
				return "", err
			}
			entry.Type = "conf"
			manifest.Files[archName] = entry
		}
	}

	// Add alias config files
	for _, ac := range params.AliasConfs {
		if _, err := os.Stat(ac); err == nil {
			archName := "conf/" + filepath.Base(ac)
			entry, err := addFileToTar(tw, ac, archName)
			if err != nil {
				return "", err
			}
			entry.Type = "conf"
			manifest.Files[archName] = entry
		}
	}

	// Marshal and add manifest as the last entry
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("archive: marshal manifest: %w", err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name:    "manifest.json",
		Size:    int64(len(manifestJSON)),
		Mode:    0644,
		ModTime: time.Now(),
	}); err != nil {
		return "", fmt.Errorf("archive: write manifest header: %w", err)
	}
	if _, err := tw.Write(manifestJSON); err != nil {
		return "", fmt.Errorf("archive: write manifest: %w", err)
	}

	return archivePath, nil
}

// addFileToTar adds a single file to the tar archive with the given archive name,
// computing its SHA-256 while writing.
func addFileToTar(tw *tar.Writer, srcPath, archName string) (FileEntry, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return FileEntry{}, fmt.Errorf("archive: open %s: %w", srcPath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return FileEntry{}, fmt.Errorf("archive: stat %s: %w", srcPath, err)
	}

	// Use forward slashes in tar paths
	archName = strings.ReplaceAll(archName, "\\", "/")

	if err := tw.WriteHeader(&tar.Header{
		Name:    archName,
		Size:    info.Size(),
		Mode:    0644,
		ModTime: info.ModTime(),
	}); err != nil {
		return FileEntry{}, fmt.Errorf("archive: header %s: %w", archName, err)
	}

	h := sha256.New()
	written, err := io.Copy(tw, io.TeeReader(f, h))
	if err != nil {
		return FileEntry{}, fmt.Errorf("archive: write %s: %w", archName, err)
	}

	return FileEntry{
		SHA256: hex.EncodeToString(h.Sum(nil)),
		Size:   written,
	}, nil
}

// addDirToTar recursively adds all files in a directory to the tar archive.
func addDirToTar(tw *tar.Writer, srcDir, archPrefix string) (map[string]FileEntry, error) {
	entries := make(map[string]FileEntry)
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		archName := archPrefix + "/" + filepath.ToSlash(rel)
		entry, err := addFileToTar(tw, path, archName)
		if err != nil {
			return err
		}
		entries[archName] = entry
		return nil
	})
	return entries, err
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
