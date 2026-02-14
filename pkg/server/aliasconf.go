package server

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// AliasConfig holds parsed alias configuration from goTinyAlias.conf.
type AliasConfig struct {
	CommandAliases map[string]string // alias -> target (may include /switch)
	FlagAliases    map[string]string // alias -> canonical flag name
	FuncAliases    map[string]string // alias -> canonical function name
	AttrAliases    map[string]string // alias -> canonical attr name
	PowerAliases   map[string]string // alias -> canonical power name
	BadNames       []string          // forbidden player names
}

// LoadAliasConfig parses one or more alias config files and merges them.
func LoadAliasConfig(paths ...string) (*AliasConfig, error) {
	ac := &AliasConfig{
		CommandAliases: make(map[string]string),
		FlagAliases:    make(map[string]string),
		FuncAliases:    make(map[string]string),
		AttrAliases:    make(map[string]string),
		PowerAliases:   make(map[string]string),
	}

	for _, path := range paths {
		if err := ac.loadFile(path); err != nil {
			return nil, fmt.Errorf("loading %s: %w", path, err)
		}
	}
	return ac, nil
}

func (ac *AliasConfig) loadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		directive := strings.ToLower(fields[0])
		switch directive {
		case "alias":
			if len(fields) < 3 {
				log.Printf("aliasconf: %s:%d: alias requires 2 arguments", path, lineNo)
				continue
			}
			ac.CommandAliases[strings.ToLower(fields[1])] = fields[2]

		case "flag_alias":
			if len(fields) < 3 {
				continue
			}
			ac.FlagAliases[strings.ToLower(fields[1])] = strings.ToLower(fields[2])

		case "function_alias":
			if len(fields) < 3 {
				continue
			}
			ac.FuncAliases[strings.ToLower(fields[1])] = strings.ToLower(fields[2])

		case "attr_alias":
			if len(fields) < 3 {
				continue
			}
			ac.AttrAliases[strings.ToLower(fields[1])] = strings.ToLower(fields[2])

		case "power_alias":
			if len(fields) < 3 {
				continue
			}
			ac.PowerAliases[strings.ToLower(fields[1])] = strings.ToLower(fields[2])

		case "bad_name":
			if len(fields) < 2 {
				continue
			}
			ac.BadNames = append(ac.BadNames, strings.ToLower(fields[1]))

		default:
			// Ignore unknown directives (e.g. config settings in compat.conf)
		}
	}
	return scanner.Err()
}

// ApplyAliasConfig applies a loaded alias config to the game.
func (g *Game) ApplyAliasConfig(ac *AliasConfig) {
	cmdCount, flagCount, funcCount, attrCount := 0, 0, 0, 0

	// Command aliases
	for alias, target := range ac.CommandAliases {
		// Target may contain /switch, e.g. "@dolist/now"
		targetCmd := target
		var prependSwitches []string
		if slashIdx := strings.IndexByte(target, '/'); slashIdx >= 0 {
			targetCmd = target[:slashIdx]
			prependSwitches = strings.Split(target[slashIdx+1:], "/")
		}

		// Resolve target command
		cmd, ok := g.Commands[strings.ToLower(targetCmd)]
		if !ok {
			log.Printf("aliasconf: command alias %q -> %q: target command %q not found", alias, target, targetCmd)
			continue
		}

		if len(prependSwitches) > 0 {
			// Create a wrapper handler that prepends the switches
			origHandler := cmd.Handler
			sw := prependSwitches // capture for closure
			g.Commands[alias] = &Command{
				Name: cmd.Name,
				Handler: func(g *Game, d *Descriptor, args string, switches []string) {
					origHandler(g, d, args, append(sw, switches...))
				},
			}
		} else {
			g.Commands[alias] = cmd
		}
		cmdCount++
	}

	// Flag aliases
	for alias, target := range ac.FlagAliases {
		targetUpper := strings.ToUpper(target)
		if def, ok := FlagTable[targetUpper]; ok {
			FlagTable[strings.ToUpper(alias)] = def
			flagCount++
		} else {
			log.Printf("aliasconf: flag alias %q -> %q: target flag not found", alias, target)
		}
	}

	// Function aliases - store for later application during eval context creation
	for alias, target := range ac.FuncAliases {
		if g.FuncAliases == nil {
			g.FuncAliases = make(map[string]string)
		}
		g.FuncAliases[strings.ToUpper(alias)] = strings.ToUpper(target)
		funcCount++
	}

	// Attr aliases
	for alias, target := range ac.AttrAliases {
		targetUpper := strings.ToUpper(target)
		// Look up the target attr
		for num, name := range gamedb.WellKnownAttrs {
			if strings.EqualFold(name, targetUpper) {
				gamedb.WellKnownAttrs[num] = name // ensure canonical casing
				// Add the alias as a well-known attr pointing to same number
				aliasUpper := strings.ToUpper(alias)
				// Check if not already defined
				found := false
				for _, n := range gamedb.WellKnownAttrs {
					if strings.EqualFold(n, aliasUpper) {
						found = true
						break
					}
				}
				if !found {
					// Register as an additional name mapping in AttrByName
					if g.DB.AttrByName == nil {
						g.DB.AttrByName = make(map[string]*gamedb.AttrDef)
					}
					g.DB.AttrByName[aliasUpper] = &gamedb.AttrDef{
						Number: num,
						Name:   aliasUpper,
					}
				}
				attrCount++
				break
			}
		}
		// Also check user-defined attrs
		if def, ok := g.DB.AttrByName[targetUpper]; ok {
			g.DB.AttrByName[strings.ToUpper(alias)] = def
			attrCount++
		}
	}

	// Power aliases - store for future use
	// (powers not yet implemented, just log the count)

	// Bad names
	g.BadNames = append(g.BadNames, ac.BadNames...)

	log.Printf("Alias config applied: %d command aliases, %d flag aliases, %d function aliases, %d attr aliases, %d bad names",
		cmdCount, flagCount, funcCount, attrCount, len(ac.BadNames))
}

// IsBadName checks if a player name is forbidden.
func (g *Game) IsBadName(name string) bool {
	lower := strings.ToLower(name)
	for _, bad := range g.BadNames {
		if wildMatchSimple(bad, lower) {
			return true
		}
	}
	return false
}

// ApplyFuncAliases applies function aliases to an eval context.
// Call this after RegisterAll.
func (g *Game) ApplyFuncAliases(ctx *eval.EvalContext) {
	for alias, target := range g.FuncAliases {
		ctx.AliasFunction(alias, target)
	}
}

// MakeEvalContextWithAliases creates an eval context with function aliases applied.
func MakeEvalContextWithAliases(g *Game, player gamedb.DBRef) *eval.EvalContext {
	ctx := MakeEvalContextWithGame(g, player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	g.ApplyFuncAliases(ctx)
	return ctx
}
