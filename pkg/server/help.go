package server

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

// HelpFile holds parsed help entries from a TinyMUSH-format help text file.
// Entries are separated by lines starting with "& topicname".
type HelpFile struct {
	Entries map[string]string // lowercase topic -> text content
}

// LoadHelpFile parses a TinyMUSH help .txt file and returns a HelpFile.
// Returns nil if the file cannot be opened.
func LoadHelpFile(path string) *HelpFile {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	hf := &HelpFile{Entries: make(map[string]string)}
	scanner := bufio.NewScanner(f)

	// Topics can have multiple "& TOPIC" aliases (e.g. & ESCAPE() / & NESCAPE())
	// that all share the same content body. Collect all topic names for each entry.
	var currentTopics []string
	var buf strings.Builder

	saveEntry := func() {
		if len(currentTopics) == 0 {
			return
		}
		text := strings.TrimRight(buf.String(), "\n ")
		for _, topic := range currentTopics {
			hf.Entries[strings.ToLower(topic)] = text
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "& ") {
			topic := strings.TrimSpace(line[2:])
			if buf.Len() == 0 && len(currentTopics) > 0 {
				// Another alias for the same entry (no content yet)
				currentTopics = append(currentTopics, topic)
			} else {
				// New entry — save the previous one
				saveEntry()
				currentTopics = []string{topic}
				buf.Reset()
			}
		} else {
			if len(currentTopics) > 0 {
				buf.WriteString(line)
				buf.WriteByte('\n')
			}
		}
	}
	// Save last entry
	saveEntry()

	return hf
}

// Lookup finds a help entry by topic name. Tries exact match first,
// then prefix match (e.g. "help @swi" matches "@switch").
// If the topic contains wildcards (* or ?), returns a list of matching topics.
func (hf *HelpFile) Lookup(topic string) string {
	topic = strings.ToLower(strings.TrimSpace(topic))
	if topic == "" {
		topic = "help"
	}

	// Wildcard search
	if strings.ContainsAny(topic, "*?") {
		var matches []string
		for key := range hf.Entries {
			if wildMatchSimple(topic, key) {
				matches = append(matches, key)
			}
		}
		if len(matches) == 0 {
			return ""
		}
		sort.Strings(matches)
		return fmt.Sprintf("Here are the entries which match '%s':\n  %s",
			topic, strings.Join(matches, "  "))
	}

	// Exact match
	if text, ok := hf.Entries[topic]; ok {
		return text
	}

	// Prefix match — find the shortest key that starts with topic
	var bestKey string
	for key := range hf.Entries {
		if strings.HasPrefix(key, topic) {
			if bestKey == "" || len(key) < len(bestKey) {
				bestKey = key
			}
		}
	}
	if bestKey != "" {
		return hf.Entries[bestKey]
	}

	return ""
}

// LoadHelpFiles loads all help files from the text directory into the Game.
func (g *Game) LoadHelpFiles(textDir string) {
	load := func(name string) *HelpFile {
		path := textDir + "/" + name
		hf := LoadHelpFile(path)
		if hf != nil {
			log.Printf("Loaded help file %s: %d entries", name, len(hf.Entries))
		}
		return hf
	}

	g.HelpMain = load("help.txt")
	g.HelpQuick = load("qhelp.txt")
	g.HelpWiz = load("wizhelp.txt")
	g.HelpNews = load("news.txt")
	g.HelpPlus = load("plushelp.txt")
	g.HelpMan = load("mushman.txt")
	g.HelpWizNews = load("wiznews.txt")
	g.HelpJobs = load("jhelp.txt")
}

// --- Help commands ---

func cmdHelp(g *Game, d *Descriptor, args string, _ []string) {
	if g.HelpMain == nil {
		d.Send("No help available.")
		return
	}
	if args == "" {
		args = "help"
	}
	text := g.HelpMain.Lookup(args)
	if text == "" {
		d.Send(fmt.Sprintf("No entry for '%s'.", args))
		return
	}
	d.Send(text)
}

func cmdQhelp(g *Game, d *Descriptor, args string, _ []string) {
	if g.HelpQuick == nil {
		d.Send("No quick help available.")
		return
	}
	if args == "" {
		args = "help"
	}
	text := g.HelpQuick.Lookup(args)
	if text == "" {
		d.Send(fmt.Sprintf("No entry for '%s'.", args))
		return
	}
	d.Send(text)
}

func cmdWizhelp(g *Game, d *Descriptor, args string, _ []string) {
	// Wizard-only
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	if g.HelpWiz == nil {
		d.Send("No wizard help available.")
		return
	}
	if args == "" {
		args = "help"
	}
	text := g.HelpWiz.Lookup(args)
	if text == "" {
		d.Send(fmt.Sprintf("No entry for '%s'.", args))
		return
	}
	d.Send(text)
}

func cmdNews(g *Game, d *Descriptor, args string, _ []string) {
	if g.HelpNews == nil {
		d.Send("No news available.")
		return
	}
	if args == "" {
		args = "help"
	}
	text := g.HelpNews.Lookup(args)
	if text == "" {
		d.Send(fmt.Sprintf("No entry for '%s'.", args))
		return
	}
	d.Send(text)
}

func cmdPlusHelp(g *Game, d *Descriptor, args string, _ []string) {
	if g.HelpPlus == nil {
		d.Send("No +help available.")
		return
	}
	if args == "" {
		args = "help"
	}
	text := g.HelpPlus.Lookup(args)
	if text == "" {
		d.Send(fmt.Sprintf("No entry for '%s'.", args))
		return
	}
	d.Send(text)
}

func cmdMan(g *Game, d *Descriptor, args string, _ []string) {
	if g.HelpMan == nil {
		d.Send("No manual available.")
		return
	}
	if args == "" {
		args = "help"
	}
	text := g.HelpMan.Lookup(args)
	if text == "" {
		d.Send(fmt.Sprintf("No entry for '%s'.", args))
		return
	}
	d.Send(text)
}

func cmdWizNews(g *Game, d *Descriptor, args string, _ []string) {
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	if g.HelpWizNews == nil {
		d.Send("No wizard news available.")
		return
	}
	if args == "" {
		args = "help"
	}
	text := g.HelpWizNews.Lookup(args)
	if text == "" {
		d.Send(fmt.Sprintf("No entry for '%s'.", args))
		return
	}
	d.Send(text)
}

func cmdJhelp(g *Game, d *Descriptor, args string, _ []string) {
	if g.HelpJobs == nil {
		d.Send("No +jhelp available.")
		return
	}
	if args == "" {
		args = "help"
	}
	text := g.HelpJobs.Lookup(args)
	if text == "" {
		d.Send(fmt.Sprintf("No entry for '%s'.", args))
		return
	}
	d.Send(text)
}
