package admin

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AliasConfig represents the structured form of alias configuration directives.
type AliasConfig struct {
	CommandAliases  map[string]string `yaml:"command_aliases,omitempty"`
	FlagAliases     map[string]string `yaml:"flag_aliases,omitempty"`
	FunctionAliases map[string]string `yaml:"function_aliases,omitempty"`
	AttrAliases     map[string]string `yaml:"attr_aliases,omitempty"`
	PowerAliases    map[string]string `yaml:"power_aliases,omitempty"`
	BadNames        []string          `yaml:"bad_names,omitempty"`
}

// ConvertAliasConf reads a legacy alias .conf file and produces YAML bytes.
func ConvertAliasConf(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open alias conf: %w", err)
	}
	defer f.Close()

	ac := AliasConfig{
		CommandAliases:  make(map[string]string),
		FlagAliases:     make(map[string]string),
		FunctionAliases: make(map[string]string),
		AttrAliases:     make(map[string]string),
		PowerAliases:    make(map[string]string),
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Skip @ directives
		if strings.HasPrefix(line, "@") {
			continue
		}

		key, val := splitKeyVal(line)
		key = strings.ToLower(key)

		switch key {
		case "alias":
			// Format: "alias <shortname> <fullname>"
			parts := strings.Fields(val)
			if len(parts) >= 2 {
				ac.CommandAliases[parts[0]] = strings.Join(parts[1:], " ")
			}
		case "flag_alias":
			// Format: "flag_alias <shortname> <fullname>"
			parts := strings.Fields(val)
			if len(parts) >= 2 {
				ac.FlagAliases[parts[0]] = parts[1]
			}
		case "function_alias":
			// Format: "function_alias <shortname> <fullname>"
			parts := strings.Fields(val)
			if len(parts) >= 2 {
				ac.FunctionAliases[parts[0]] = parts[1]
			}
		case "attr_alias":
			// Format: "attr_alias <shortname> <fullname>"
			parts := strings.Fields(val)
			if len(parts) >= 2 {
				ac.AttrAliases[parts[0]] = parts[1]
			}
		case "power_alias":
			// Format: "power_alias <shortname> <fullname>"
			parts := strings.Fields(val)
			if len(parts) >= 2 {
				ac.PowerAliases[parts[0]] = parts[1]
			}
		case "bad_name":
			if val != "" {
				ac.BadNames = append(ac.BadNames, val)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read alias conf: %w", err)
	}

	// Clean up empty maps
	if len(ac.CommandAliases) == 0 {
		ac.CommandAliases = nil
	}
	if len(ac.FlagAliases) == 0 {
		ac.FlagAliases = nil
	}
	if len(ac.FunctionAliases) == 0 {
		ac.FunctionAliases = nil
	}
	if len(ac.AttrAliases) == 0 {
		ac.AttrAliases = nil
	}
	if len(ac.PowerAliases) == 0 {
		ac.PowerAliases = nil
	}

	data, err := yaml.Marshal(ac)
	if err != nil {
		return nil, fmt.Errorf("marshal alias yaml: %w", err)
	}
	return data, nil
}
