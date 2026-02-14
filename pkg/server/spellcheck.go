package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

// SpellChecker provides dictionary-based spellchecking with optional remote API fallback.
type SpellChecker struct {
	mu          sync.RWMutex
	baseWords   map[string]bool // base dictionary (lowercase)
	learned     map[string]bool // dynamically learned (lowercase)
	learnedPath string          // path to learned.txt
	apiURL      string          // LanguageTool API URL (empty = no remote)
	enabled     bool
	httpClient  *http.Client
}

// SpellIssue represents a spelling or grammar issue found in text.
type SpellIssue struct {
	Offset  int    // byte offset in original text
	Length  int    // length of the problematic span
	Word    string // the problematic word/phrase
	IsGrammar bool // true = grammar issue, false = spelling issue
}

// NewSpellChecker creates a SpellChecker, loading dictionaries from dictDir.
func NewSpellChecker(dictDir, apiURL string, enabled bool) *SpellChecker {
	sc := &SpellChecker{
		baseWords: make(map[string]bool),
		learned:   make(map[string]bool),
		apiURL:    apiURL,
		enabled:   enabled,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	// Create dictDir if it doesn't exist
	if err := os.MkdirAll(dictDir, 0755); err != nil {
		log.Printf("spellcheck: cannot create dict dir %s: %v", dictDir, err)
		return sc
	}

	// Load base dictionary
	basePath := filepath.Join(dictDir, "base.txt")
	if count, err := sc.loadDictFile(basePath, sc.baseWords); err != nil {
		log.Printf("spellcheck: base dict %s: %v", basePath, err)
	} else {
		log.Printf("spellcheck: loaded %d base words from %s", count, basePath)
	}

	// Load learned dictionary
	sc.learnedPath = filepath.Join(dictDir, "learned.txt")
	if count, err := sc.loadDictFile(sc.learnedPath, sc.learned); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("spellcheck: learned dict %s: %v", sc.learnedPath, err)
		}
	} else {
		log.Printf("spellcheck: loaded %d learned words from %s", count, sc.learnedPath)
	}

	return sc
}

// loadDictFile reads one word per line into the target map. Returns count loaded.
func (sc *SpellChecker) loadDictFile(path string, target map[string]bool) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word == "" || word[0] == '#' {
			continue
		}
		target[strings.ToLower(word)] = true
		count++
	}
	return count, scanner.Err()
}

// IsKnown returns true if the word is recognized (in custom, base, learned, or remote).
func (sc *SpellChecker) IsKnown(word string, custom map[string]bool) bool {
	lower := strings.ToLower(word)

	// Check custom words first
	if custom != nil && custom[lower] {
		return true
	}

	sc.mu.RLock()
	inBase := sc.baseWords[lower]
	inLearned := sc.learned[lower]
	sc.mu.RUnlock()

	if inBase || inLearned {
		return true
	}

	// Try remote API if configured (word-level check)
	if sc.apiURL != "" {
		if sc.queryRemoteWord(word) {
			sc.LearnWord(word)
			return true
		}
		return false
	}

	return false
}

// CheckText returns a list of misspelled words in text.
func (sc *SpellChecker) CheckText(text string, custom map[string]bool) []string {
	if !sc.enabled {
		return nil
	}
	tokens := tokenize(text)
	var misspelled []string
	seen := make(map[string]bool)
	for _, tok := range tokens {
		word := tok.cleaned
		if word == "" || isNumericOrSpecial(word) {
			continue
		}
		lower := strings.ToLower(word)
		if seen[lower] {
			continue
		}
		if !sc.IsKnown(word, custom) {
			misspelled = append(misspelled, word)
			seen[lower] = true
		}
	}
	return misspelled
}

// CheckTextWithGrammar returns spelling and grammar issues using the remote API.
// Falls back to local spelling-only check if no API is configured.
func (sc *SpellChecker) CheckTextWithGrammar(text string, custom map[string]bool) []SpellIssue {
	if !sc.enabled {
		return nil
	}

	// If we have an API, use full-text grammar check
	if sc.apiURL != "" {
		issues := sc.queryRemoteFullText(text)
		// Filter out issues for custom/known words (spelling only)
		var filtered []SpellIssue
		for _, issue := range issues {
			if !issue.IsGrammar {
				// For spelling issues, check if word is in custom dict
				lower := strings.ToLower(issue.Word)
				if custom != nil && custom[lower] {
					continue
				}
				sc.mu.RLock()
				known := sc.baseWords[lower] || sc.learned[lower]
				sc.mu.RUnlock()
				if known {
					continue
				}
			}
			filtered = append(filtered, issue)
		}
		return filtered
	}

	// No API: spelling-only via local dict
	tokens := tokenize(text)
	var issues []SpellIssue
	for _, tok := range tokens {
		if tok.cleaned == "" || isNumericOrSpecial(tok.cleaned) {
			continue
		}
		if !sc.IsKnown(tok.cleaned, custom) {
			issues = append(issues, SpellIssue{
				Offset: tok.wordStart,
				Length: tok.wordEnd - tok.wordStart,
				Word:   tok.cleaned,
			})
		}
	}
	return issues
}

// HighlightText returns text with misspelled words highlighted.
// If useAnsi is true, uses ANSI red underline (\033[4;31m).
// If useAnsi is false, wraps misspelled words with >word<.
func (sc *SpellChecker) HighlightText(text string, custom map[string]bool, useAnsi bool) string {
	if !sc.enabled {
		return text
	}
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return text
	}

	// Build a set of misspelled words
	misspelled := make(map[string]bool)
	for _, tok := range tokens {
		if tok.cleaned == "" || isNumericOrSpecial(tok.cleaned) {
			continue
		}
		if !sc.IsKnown(tok.cleaned, custom) {
			misspelled[strings.ToLower(tok.cleaned)] = true
		}
	}
	if len(misspelled) == 0 {
		return text
	}

	return highlightTokens(text, tokens, misspelled, useAnsi, false)
}

// HighlightTextWithGrammar returns text with spelling issues in red underline
// and grammar issues in cyan. Falls back to local dict if no API.
func (sc *SpellChecker) HighlightTextWithGrammar(text string, custom map[string]bool, useAnsi bool) string {
	if !sc.enabled {
		return text
	}

	issues := sc.CheckTextWithGrammar(text, custom)
	if len(issues) == 0 {
		return text
	}

	// Build highlighted text from issues (sorted by offset)
	var buf strings.Builder
	lastEnd := 0
	for _, issue := range issues {
		if issue.Offset < lastEnd {
			continue // overlapping issue, skip
		}
		// Write text before this issue
		if issue.Offset > lastEnd {
			buf.WriteString(text[lastEnd:issue.Offset])
		}
		end := issue.Offset + issue.Length
		if end > len(text) {
			end = len(text)
		}
		span := text[issue.Offset:end]
		if useAnsi {
			if issue.IsGrammar {
				// Cyan underline for grammar
				buf.WriteString("\033[4;36m")
			} else {
				// Red underline for spelling
				buf.WriteString("\033[4;31m")
			}
			buf.WriteString(span)
			buf.WriteString("\033[0m")
		} else {
			if issue.IsGrammar {
				buf.WriteByte('{')
				buf.WriteString(span)
				buf.WriteByte('}')
			} else {
				buf.WriteByte('>')
				buf.WriteString(span)
				buf.WriteByte('<')
			}
		}
		lastEnd = end
	}
	if lastEnd < len(text) {
		buf.WriteString(text[lastEnd:])
	}
	return buf.String()
}

// highlightTokens rebuilds text with misspelled tokens highlighted.
func highlightTokens(text string, tokens []token, misspelled map[string]bool, useAnsi bool, isGrammar bool) string {
	var buf strings.Builder
	lastEnd := 0
	for _, tok := range tokens {
		if tok.start > lastEnd {
			buf.WriteString(text[lastEnd:tok.start])
		}
		if tok.cleaned != "" && misspelled[strings.ToLower(tok.cleaned)] {
			// Write leading punctuation
			buf.WriteString(text[tok.start:tok.wordStart])
			if useAnsi {
				// Red underline for spelling
				buf.WriteString("\033[4;31m")
				buf.WriteString(text[tok.wordStart:tok.wordEnd])
				buf.WriteString("\033[0m")
			} else {
				// Non-ANSI: >word<
				buf.WriteByte('>')
				buf.WriteString(text[tok.wordStart:tok.wordEnd])
				buf.WriteByte('<')
			}
			// Write trailing punctuation
			buf.WriteString(text[tok.wordEnd:tok.end])
		} else {
			buf.WriteString(text[tok.start:tok.end])
		}
		lastEnd = tok.end
	}
	if lastEnd < len(text) {
		buf.WriteString(text[lastEnd:])
	}
	return buf.String()
}

// LearnWord adds a word to the learned dictionary and persists it.
func (sc *SpellChecker) LearnWord(word string) {
	lower := strings.ToLower(word)
	sc.mu.Lock()
	if sc.learned[lower] {
		sc.mu.Unlock()
		return
	}
	sc.learned[lower] = true
	sc.mu.Unlock()

	// Append to file
	if sc.learnedPath != "" {
		f, err := os.OpenFile(sc.learnedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("spellcheck: cannot write learned word: %v", err)
			return
		}
		fmt.Fprintln(f, lower)
		f.Close()
	}
}

// queryRemoteWord checks a single word against the LanguageTool API.
// Returns true if the word is valid (no spelling errors).
func (sc *SpellChecker) queryRemoteWord(word string) bool {
	if sc.apiURL == "" {
		return false
	}

	time.Sleep(200 * time.Millisecond)

	data := url.Values{}
	data.Set("text", word)
	data.Set("language", "en-US")

	resp, err := sc.httpClient.PostForm(sc.apiURL, data)
	if err != nil {
		return true // Network error: treat as valid
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return true
	}

	var result ltResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return true
	}

	return len(result.Matches) == 0
}

// ltResponse is the LanguageTool API response structure.
type ltResponse struct {
	Matches []ltMatch `json:"matches"`
}

type ltMatch struct {
	Message string `json:"message"`
	Offset  int    `json:"offset"`
	Length  int    `json:"length"`
	Rule    ltRule `json:"rule"`
}

type ltRule struct {
	ID       string     `json:"id"`
	Category ltCategory `json:"category"`
}

type ltCategory struct {
	ID string `json:"id"`
}

// queryRemoteFullText sends full text to LanguageTool for spelling + grammar checking.
func (sc *SpellChecker) queryRemoteFullText(text string) []SpellIssue {
	if sc.apiURL == "" {
		return nil
	}

	time.Sleep(200 * time.Millisecond)

	data := url.Values{}
	data.Set("text", text)
	data.Set("language", "en-US")

	resp, err := sc.httpClient.PostForm(sc.apiURL, data)
	if err != nil {
		return nil // Network error: no issues
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var result ltResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	var issues []SpellIssue
	for _, m := range result.Matches {
		end := m.Offset + m.Length
		if end > len(text) {
			end = len(text)
		}
		word := ""
		if m.Offset < len(text) {
			word = text[m.Offset:end]
		}

		isGrammar := true
		cat := strings.ToUpper(m.Rule.Category.ID)
		if cat == "TYPOS" || cat == "SPELLING" || strings.Contains(strings.ToUpper(m.Rule.ID), "SPELL") {
			isGrammar = false
		}

		issues = append(issues, SpellIssue{
			Offset:    m.Offset,
			Length:    m.Length,
			Word:      word,
			IsGrammar: isGrammar,
		})
	}
	return issues
}

// token represents a whitespace-delimited token in the input text.
type token struct {
	start     int    // start index in original text (including leading punct)
	end       int    // end index in original text (including trailing punct)
	wordStart int    // start of the actual word (after leading punct)
	wordEnd   int    // end of the actual word (before trailing punct)
	cleaned   string // the word with punctuation stripped
}

// tokenize splits text on whitespace and records positions for reconstruction.
func tokenize(text string) []token {
	var tokens []token
	i := 0
	for i < len(text) {
		// Skip whitespace
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}
		if i >= len(text) {
			break
		}
		// Find end of token
		start := i
		for i < len(text) && text[i] != ' ' && text[i] != '\t' && text[i] != '\n' && text[i] != '\r' {
			i++
		}
		end := i

		// Strip leading/trailing punctuation to find the word
		ws := start
		we := end
		for ws < we && isPunct(rune(text[ws])) {
			ws++
		}
		for we > ws && isPunct(rune(text[we-1])) {
			we--
		}

		cleaned := ""
		if ws < we {
			cleaned = text[ws:we]
		}

		tokens = append(tokens, token{
			start:     start,
			end:       end,
			wordStart: ws,
			wordEnd:   we,
			cleaned:   cleaned,
		})
	}
	return tokens
}

// isPunct returns true for common punctuation that should be stripped from words.
func isPunct(r rune) bool {
	return unicode.IsPunct(r) || r == '"' || r == '\'' || r == '(' || r == ')' || r == '[' || r == ']'
}

// isNumericOrSpecial returns true for tokens that shouldn't be spellchecked.
func isNumericOrSpecial(s string) bool {
	if s == "" {
		return true
	}
	if s[0] == '#' {
		return true
	}
	allDigit := true
	for _, r := range s {
		if !unicode.IsDigit(r) && r != '.' && r != '-' && r != '+' {
			allDigit = false
			break
		}
	}
	return allDigit
}
