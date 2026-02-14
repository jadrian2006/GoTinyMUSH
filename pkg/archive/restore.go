package archive

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RestoreParams holds all inputs needed to restore an archive.
type RestoreParams struct {
	ArchivePath string    // Path to the .tar.gz archive
	BoltDest    string    // Destination path for bolt database
	SQLDest     string    // Destination path for SQLite database (empty = skip)
	DictDest    string    // Destination directory for dictionary files (empty = skip)
	TextDest    string    // Destination directory for text files (empty = skip)
	ConfDest    string    // Destination path for main config file (empty = skip)
	AliasDest   string    // Destination directory for alias config files (empty = skip)
	Stdin       io.Reader // For interactive prompts
	Stdout      io.Writer // For interactive output
}

// RestoreResult summarizes a completed restore operation.
type RestoreResult struct {
	FilesRestored int
	Warnings      []string
}

// RestoreArchive extracts and validates an archive, restoring files to their destinations.
func RestoreArchive(params RestoreParams) (*RestoreResult, error) {
	result := &RestoreResult{}

	// Create temp dir for extraction
	tmpDir, err := os.MkdirTemp("", "mush-restore-*")
	if err != nil {
		return nil, fmt.Errorf("restore: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract all entries
	if err := extractArchive(params.ArchivePath, tmpDir); err != nil {
		return nil, fmt.Errorf("restore: extract: %w", err)
	}

	// Parse manifest
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("restore: manifest.json not found in archive")
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("restore: parse manifest: %w", err)
	}

	// Validate checksums
	for archName, entry := range manifest.Files {
		extractedPath := filepath.Join(tmpDir, filepath.FromSlash(archName))
		ok, err := validateChecksum(extractedPath, entry.SHA256)
		if err != nil {
			return nil, fmt.Errorf("restore: checksum %s: %w", archName, err)
		}
		if !ok {
			return nil, fmt.Errorf("restore: checksum mismatch for %s — archive may be corrupt", archName)
		}
	}

	// Restore bolt database
	boltSrc := filepath.Join(tmpDir, "data", "game.bolt")
	if _, err := os.Stat(boltSrc); err == nil && params.BoltDest != "" {
		if err := os.MkdirAll(filepath.Dir(params.BoltDest), 0755); err != nil {
			return nil, fmt.Errorf("restore: create bolt dir: %w", err)
		}
		if err := copyFile(boltSrc, params.BoltDest); err != nil {
			return nil, fmt.Errorf("restore: copy bolt: %w", err)
		}
		result.FilesRestored++
	}

	// Restore SQL database
	sqlSrc := filepath.Join(tmpDir, "data", "game.sqldb")
	if _, err := os.Stat(sqlSrc); err == nil && params.SQLDest != "" {
		if err := os.MkdirAll(filepath.Dir(params.SQLDest), 0755); err != nil {
			return nil, fmt.Errorf("restore: create sql dir: %w", err)
		}
		if err := copyFile(sqlSrc, params.SQLDest); err != nil {
			return nil, fmt.Errorf("restore: copy sql: %w", err)
		}
		result.FilesRestored++
	}

	// Restore dictionary files
	dictSrc := filepath.Join(tmpDir, "data", "dict")
	if info, err := os.Stat(dictSrc); err == nil && info.IsDir() && params.DictDest != "" {
		if err := os.MkdirAll(params.DictDest, 0755); err != nil {
			return nil, fmt.Errorf("restore: create dict dir: %w", err)
		}
		n, err := copyDir(dictSrc, params.DictDest)
		if err != nil {
			return nil, fmt.Errorf("restore: copy dict: %w", err)
		}
		result.FilesRestored += n
	}

	// Restore text files
	textSrc := filepath.Join(tmpDir, "text")
	if info, err := os.Stat(textSrc); err == nil && info.IsDir() && params.TextDest != "" {
		if err := os.MkdirAll(params.TextDest, 0755); err != nil {
			return nil, fmt.Errorf("restore: create text dir: %w", err)
		}
		n, err := copyDir(textSrc, params.TextDest)
		if err != nil {
			return nil, fmt.Errorf("restore: copy text: %w", err)
		}
		result.FilesRestored += n
	}

	// Restore config files with interactive diff
	confSrc := filepath.Join(tmpDir, "conf")
	if info, err := os.Stat(confSrc); err == nil && info.IsDir() {
		entries, err := os.ReadDir(confSrc)
		if err != nil {
			return nil, fmt.Errorf("restore: read conf dir: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			srcFile := filepath.Join(confSrc, entry.Name())
			var destFile string

			// Main config goes to ConfDest, alias configs go to AliasDest
			if params.ConfDest != "" && entry.Name() == filepath.Base(params.ConfDest) {
				destFile = params.ConfDest
			} else if params.AliasDest != "" {
				destFile = filepath.Join(params.AliasDest, entry.Name())
			} else {
				continue
			}

			action, err := promptConfigDiff(srcFile, destFile, entry.Name(), params.Stdin, params.Stdout)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("config prompt error for %s: %v", entry.Name(), err))
				continue
			}
			switch action {
			case 'U':
				if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
					return nil, fmt.Errorf("restore: create conf dir: %w", err)
				}
				if err := copyFile(srcFile, destFile); err != nil {
					return nil, fmt.Errorf("restore: copy conf %s: %w", entry.Name(), err)
				}
				result.FilesRestored++
			case 'K', 'S':
				result.Warnings = append(result.Warnings, fmt.Sprintf("kept current config: %s", entry.Name()))
			}
		}
	}

	return result, nil
}

// extractArchive extracts a .tar.gz to a destination directory.
func extractArchive(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Sanitize path to prevent directory traversal
		target := filepath.Join(destDir, filepath.FromSlash(hdr.Name))
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("invalid archive entry: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

// validateChecksum checks a file's SHA-256 against the expected hex string.
func validateChecksum(path, expected string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	return actual == expected, nil
}

// promptConfigDiff handles interactive config file comparison during restore.
// Returns 'U' (use archived), 'K' (keep current), or 'S' (skip).
func promptConfigDiff(srcFile, destFile, name string, stdin io.Reader, stdout io.Writer) (byte, error) {
	// If destination doesn't exist, copy without prompting
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		return 'U', nil
	}

	// Compare files
	srcData, err := os.ReadFile(srcFile)
	if err != nil {
		return 0, err
	}
	destData, err := os.ReadFile(destFile)
	if err != nil {
		return 0, err
	}

	// If identical, skip
	if string(srcData) == string(destData) {
		return 'S', nil
	}

	scanner := bufio.NewScanner(stdin)
	for {
		fmt.Fprintf(stdout, "\nConfig file %q differs from archive.\n", name)
		fmt.Fprintf(stdout, "[K]eep current  [U]se archived  [D]iff  [S]kip: ")

		if !scanner.Scan() {
			return 'S', nil
		}
		input := strings.TrimSpace(strings.ToUpper(scanner.Text()))
		if input == "" {
			continue
		}

		switch input[0] {
		case 'K':
			return 'K', nil
		case 'U':
			return 'U', nil
		case 'S':
			return 'S', nil
		case 'D':
			simpleDiff(string(destData), string(srcData), stdout)
		default:
			fmt.Fprintf(stdout, "Please enter K, U, D, or S.\n")
		}
	}
}

// simpleDiff shows a basic line-by-line comparison between current and archived content.
func simpleDiff(current, archived string, w io.Writer) {
	curLines := strings.Split(current, "\n")
	arcLines := strings.Split(archived, "\n")

	fmt.Fprintf(w, "\n--- current\n+++ archived\n")

	maxLen := len(curLines)
	if len(arcLines) > maxLen {
		maxLen = len(arcLines)
	}

	for i := 0; i < maxLen; i++ {
		var curLine, arcLine string
		if i < len(curLines) {
			curLine = curLines[i]
		}
		if i < len(arcLines) {
			arcLine = arcLines[i]
		}
		if curLine != arcLine {
			if i < len(curLines) {
				fmt.Fprintf(w, "- %s\n", curLine)
			}
			if i < len(arcLines) {
				fmt.Fprintf(w, "+ %s\n", arcLine)
			}
		}
	}
	fmt.Fprintln(w)
}

// copyDir recursively copies all files from src to dst. Returns count of files copied.
func copyDir(src, dst string) (int, error) {
	count := 0
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		if err := copyFile(path, destPath); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}
