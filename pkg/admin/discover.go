package admin

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// knownTextFiles are filenames commonly used as MUSH text files.
var knownTextFiles = map[string]bool{
	"connect.txt":     true,
	"motd.txt":        true,
	"wizmotd.txt":     true,
	"quit.txt":        true,
	"register.txt":    true,
	"create_reg.txt":  true,
	"down.txt":        true,
	"full.txt":        true,
	"guest_motd.txt":  true,
	"htmlconn.txt":    true,
	"badsite.txt":     true,
	"newuser.txt":     true,
	"help.txt":        true,
	"wizhelp.txt":     true,
	"plushelp.txt":    true,
	"staffhelp.txt":  true,
	"news.txt":        true,
}

// DiscoverFiles scans a directory tree and classifies files by role.
func DiscoverFiles(dir string) ([]DiscoveredFile, error) {
	var files []DiscoveredFile

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		// Normalize to forward slashes for display
		rel = filepath.ToSlash(rel)

		// Skip manifest.json (GoTinyMUSH internal)
		if filepath.Base(rel) == "manifest.json" {
			return nil
		}

		df := classifyFile(rel, path, info)
		files = append(files, df)
		return nil
	})

	return files, err
}

// classifyFile determines the role of a single file based on heuristics.
func classifyFile(relPath, fullPath string, info os.FileInfo) DiscoveredFile {
	df := DiscoveredFile{
		Path: relPath,
		Size: info.Size(),
		Role: RoleUnknown,
	}

	baseName := filepath.Base(relPath)
	lowerBase := strings.ToLower(baseName)
	ext := strings.ToLower(filepath.Ext(baseName))
	dirName := strings.ToLower(filepath.Dir(relPath))

	// Check for flatfile by extension
	if ext == ".flat" {
		df.Role = RoleFlatfile
		df.Confidence = "high"
		df.Reason = "has .FLAT extension"
		return df
	}

	// Check for comsys by name
	if lowerBase == "mod_comsys.db" {
		df.Role = RoleComsys
		df.Confidence = "high"
		df.Reason = "named mod_comsys.db"
		return df
	}

	// Check for known text files
	if knownTextFiles[lowerBase] {
		df.Role = RoleTextFile
		df.Confidence = "high"
		df.Reason = "known MUSH text file name"
		return df
	}

	// Check for text/ subdirectory
	if strings.HasPrefix(dirName, "text") || dirName == "text" {
		if ext == ".txt" {
			df.Role = RoleTextFile
			df.Confidence = "high"
			df.Reason = ".txt file in text/ directory"
			return df
		}
	}

	// Check for dict/ subdirectory
	if strings.HasPrefix(dirName, "dict") || strings.Contains(dirName, "/dict") ||
		strings.HasPrefix(dirName, "data/dict") {
		df.Role = RoleDictFile
		df.Confidence = "high"
		df.Reason = "file in dict/ directory"
		return df
	}

	// Check for dict file names
	if lowerBase == "base.txt" || lowerBase == "learned.txt" {
		// Could be dict or text; check parent dir
		if strings.Contains(dirName, "dict") {
			df.Role = RoleDictFile
			df.Confidence = "high"
			df.Reason = "known dictionary file in dict/ directory"
		} else {
			df.Role = RoleDictFile
			df.Confidence = "medium"
			df.Reason = "known dictionary file name"
		}
		return df
	}

	// Check for config files by extension and content
	if ext == ".conf" {
		role, confidence, reason := classifyConfFile(fullPath, lowerBase)
		df.Role = role
		df.Confidence = confidence
		df.Reason = reason
		return df
	}

	if ext == ".yaml" || ext == ".yml" {
		df.Role = RoleMainConf
		df.Confidence = "medium"
		df.Reason = "YAML config file"
		return df
	}

	// Check file content for flatfile signature
	if info.Size() > 0 {
		firstLine := peekFirstLine(fullPath)
		if strings.HasPrefix(firstLine, "+T") || strings.HasPrefix(firstLine, "+X") {
			df.Role = RoleFlatfile
			df.Confidence = "medium"
			df.Reason = "file starts with flatfile header (+T or +X)"
			return df
		}
		if strings.HasPrefix(firstLine, "+V") {
			df.Role = RoleComsys
			df.Confidence = "medium"
			df.Reason = "file starts with comsys header (+V)"
			return df
		}
	}

	// Check for .txt files not in known locations
	if ext == ".txt" {
		df.Role = RoleTextFile
		df.Confidence = "low"
		df.Reason = ".txt file (unrecognized name)"
		return df
	}

	df.Confidence = "low"
	df.Reason = "unrecognized file type"
	return df
}

// classifyConfFile examines a .conf file to distinguish main config from alias config.
func classifyConfFile(fullPath, lowerBase string) (FileRole, string, string) {
	// Check filename for alias/compat hints
	if strings.Contains(lowerBase, "alias") || strings.Contains(lowerBase, "compat") {
		return RoleAliasConf, "high", "alias/compat config file by name"
	}

	// Peek at content to distinguish
	hasPort := false
	hasMudName := false
	hasAlias := false
	hasFlagAlias := false

	f, err := os.Open(fullPath)
	if err != nil {
		return RoleMainConf, "low", ".conf file (cannot read)"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	linesRead := 0
	for scanner.Scan() && linesRead < 200 {
		linesRead++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, _ := splitKeyVal(line)
		key = strings.ToLower(key)

		switch key {
		case "port":
			hasPort = true
		case "mud_name":
			hasMudName = true
		case "alias":
			hasAlias = true
		case "flag_alias":
			hasFlagAlias = true
		}
	}

	if hasPort || hasMudName {
		return RoleMainConf, "high", ".conf file with port/mud_name directives"
	}
	if hasAlias || hasFlagAlias {
		return RoleAliasConf, "high", ".conf file with alias directives"
	}

	return RoleMainConf, "medium", ".conf file (no clear indicators)"
}

// splitKeyVal splits a line on the first whitespace. Duplicated from gameconf to avoid import cycle.
func splitKeyVal(line string) (string, string) {
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			return line[:i], strings.TrimSpace(line[i+1:])
		}
	}
	return line, ""
}

// peekFirstLine reads the first non-empty line from a file.
func peekFirstLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line
		}
	}
	return ""
}
