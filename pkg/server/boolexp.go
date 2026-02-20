package server

import (
	"log"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Lock attribute numbers (from attrs.go well-known attrs)
const (
	aLock    = 42  // A_LOCK — default lock
	aFail    = 3   // A_FAIL
	aOFail   = 2   // A_OFAIL
	aAFail   = 13  // A_AFAIL
	aSucc    = 4   // A_SUCC
	aOSucc   = 1   // A_OSUCC
	aASucc   = 12  // A_ASUCC
	aDrop    = 9   // A_DROP
	aODrop   = 8   // A_ODROP
	aADrop   = 14  // A_ADROP
	aLEnter  = 59  // A_LENTER — enter lock
	aLLeave  = 60  // A_LLEAVE — leave lock
	aLUse    = 62  // A_LUSE — use lock
	aLGive   = 63  // A_LGIVE — give lock
	aLRecv   = 87  // A_LRECEIVE — receive lock
	aEFail   = 66  // A_EFAIL
	aOEFail  = 67  // A_OEFAIL
	aAEFail  = 68  // A_AEFAIL
	aLFail   = 69  // A_LFAIL
	aOLFail  = 70  // A_OLFAIL
	aALFail  = 71  // A_ALFAIL
	aUFail   = 75  // A_UFAIL
	aOUFail  = 76  // A_OUFAIL
	aAUFail  = 77  // A_AUFAIL
	aGFail   = 129 // A_GFAIL
	aOGFail  = 130 // A_OGFAIL
	aAGFail  = 131 // A_AGFAIL
	aRFail   = 132 // A_RFAIL
	aORFail  = 133 // A_ORFAIL
	aARFail  = 134 // A_ARFAIL
)

// Maximum indirection depth for @-locks to prevent infinite loops.
const maxIndirDepth = 20

// ---------- Parser ----------

// boolParser holds the state for parsing a lock string.
type boolParser struct {
	g      *Game
	player gamedb.DBRef
	src    string
	pos    int
}

func (p *boolParser) peek() byte {
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *boolParser) advance() byte {
	ch := p.peek()
	if ch != 0 {
		p.pos++
	}
	return ch
}

func (p *boolParser) skipSpaces() {
	for p.pos < len(p.src) && p.src[p.pos] == ' ' {
		p.pos++
	}
}

// ParseBoolExp parses a lock string into a BoolExp tree.
// Grammar:
//
//	E → T ('|' E)?
//	T → F ('&' T)?
//	F → '!' F | '@' L | '+' L | '=' L | '$' L | L
//	L → '(' E ')' | '#' number | name ':' pattern | name '/' pattern | name
func ParseBoolExp(g *Game, player gamedb.DBRef, lockStr string) *gamedb.BoolExp {
	lockStr = strings.TrimSpace(lockStr)
	if lockStr == "" {
		return nil
	}
	p := &boolParser{g: g, player: player, src: lockStr}
	result := p.parseE()
	return result
}

func (p *boolParser) parseE() *gamedb.BoolExp {
	left := p.parseT()
	p.skipSpaces()
	if p.peek() == '|' {
		p.advance()
		right := p.parseE()
		return &gamedb.BoolExp{Type: gamedb.BoolOr, Sub1: left, Sub2: right}
	}
	return left
}

func (p *boolParser) parseT() *gamedb.BoolExp {
	left := p.parseF()
	p.skipSpaces()
	if p.peek() == '&' {
		p.advance()
		right := p.parseT()
		return &gamedb.BoolExp{Type: gamedb.BoolAnd, Sub1: left, Sub2: right}
	}
	return left
}

func (p *boolParser) parseF() *gamedb.BoolExp {
	p.skipSpaces()
	switch p.peek() {
	case '!':
		p.advance()
		sub := p.parseF()
		return &gamedb.BoolExp{Type: gamedb.BoolNot, Sub1: sub}
	case '@':
		p.advance()
		sub := p.parseLiteral()
		if sub == nil || sub.Type != gamedb.BoolConst {
			return nil
		}
		return &gamedb.BoolExp{Type: gamedb.BoolIndir, Sub1: sub}
	case '+':
		p.advance()
		sub := p.parseLiteral()
		if sub == nil || (sub.Type != gamedb.BoolConst && sub.Type != gamedb.BoolAttr) {
			return nil
		}
		return &gamedb.BoolExp{Type: gamedb.BoolCarry, Sub1: sub}
	case '=':
		p.advance()
		sub := p.parseLiteral()
		if sub == nil || (sub.Type != gamedb.BoolConst && sub.Type != gamedb.BoolAttr) {
			return nil
		}
		return &gamedb.BoolExp{Type: gamedb.BoolIs, Sub1: sub}
	case '$':
		p.advance()
		sub := p.parseLiteral()
		if sub == nil || sub.Type != gamedb.BoolConst {
			return nil
		}
		return &gamedb.BoolExp{Type: gamedb.BoolOwner, Sub1: sub}
	default:
		return p.parseLiteral()
	}
}

func (p *boolParser) parseLiteral() *gamedb.BoolExp {
	p.skipSpaces()
	if p.peek() == '(' {
		p.advance()
		sub := p.parseE()
		p.skipSpaces()
		if p.peek() == ')' {
			p.advance()
		}
		return sub
	}

	// Collect a name token — everything up to operator chars or end
	start := p.pos
	for p.pos < len(p.src) {
		ch := p.src[p.pos]
		if ch == '&' || ch == '|' || ch == '!' || ch == '(' || ch == ')' {
			break
		}
		// Check for : or / which separate name from pattern
		if ch == ':' || ch == '/' {
			name := strings.TrimSpace(p.src[start:p.pos])
			sep := ch
			p.pos++ // skip separator
			// Collect the pattern (everything up to next operator)
			patStart := p.pos
			for p.pos < len(p.src) {
				pc := p.src[p.pos]
				if pc == '&' || pc == '|' || pc == ')' {
					break
				}
				p.pos++
			}
			pattern := strings.TrimSpace(p.src[patStart:p.pos])
			if sep == ':' {
				return &gamedb.BoolExp{Type: gamedb.BoolAttr, Thing: p.resolveAttrNum(name), StrVal: pattern}
			}
			// sep == '/'
			return &gamedb.BoolExp{Type: gamedb.BoolEval, Thing: p.resolveAttrNum(name), StrVal: pattern}
		}
		p.pos++
	}

	token := strings.TrimSpace(p.src[start:p.pos])
	if token == "" {
		return nil
	}

	// #dbref
	if token[0] == '#' {
		n, err := strconv.Atoi(token[1:])
		if err == nil {
			return &gamedb.BoolExp{Type: gamedb.BoolConst, Thing: n}
		}
	}

	// Resolve as object name
	ref := p.g.MatchObject(p.player, token)
	if ref == gamedb.Nothing {
		// Try player lookup
		ref = p.g.LookupPlayer(token)
	}
	if ref != gamedb.Nothing {
		return &gamedb.BoolExp{Type: gamedb.BoolConst, Thing: int(ref)}
	}

	// Unresolved — treat as impossible lock (nothing matches)
	log.Printf("BOOLEXP: unresolved name %q in lock", token)
	return &gamedb.BoolExp{Type: gamedb.BoolConst, Thing: int(gamedb.Nothing)}
}

// resolveAttrNum looks up an attribute name and returns its number.
func (p *boolParser) resolveAttrNum(name string) int {
	// Try numeric attr number first (from serialized locks like "547:pattern")
	if n, err := strconv.Atoi(name); err == nil && n >= 0 {
		return n
	}
	upper := strings.ToUpper(name)
	for num, n := range gamedb.WellKnownAttrs {
		if n == upper {
			return num
		}
	}
	if def, ok := p.g.DB.AttrByName[upper]; ok {
		return def.Number
	}
	return -1
}

// ---------- Evaluator ----------

// EvalBoolExp evaluates a boolean lock expression tree.
// player = the object being tested against the lock
// thing  = the object that owns the lock
// from   = the object that triggered the lock check (for eval locks)
// depth  = current indirection depth (to prevent infinite recursion)
func EvalBoolExp(g *Game, player, thing, from gamedb.DBRef, b *gamedb.BoolExp, depth int) bool {
	if b == nil {
		return true // nil lock = unlocked
	}
	if depth > maxIndirDepth {
		return false
	}

	switch b.Type {
	case gamedb.BoolAnd:
		return EvalBoolExp(g, player, thing, from, b.Sub1, depth) &&
			EvalBoolExp(g, player, thing, from, b.Sub2, depth)

	case gamedb.BoolOr:
		return EvalBoolExp(g, player, thing, from, b.Sub1, depth) ||
			EvalBoolExp(g, player, thing, from, b.Sub2, depth)

	case gamedb.BoolNot:
		return !EvalBoolExp(g, player, thing, from, b.Sub1, depth)

	case gamedb.BoolConst:
		target := gamedb.DBRef(b.Thing)
		if target == gamedb.Nothing {
			return false
		}
		// Player IS the object or CARRIES it
		if player == target {
			return true
		}
		return playerCarries(g, player, target)

	case gamedb.BoolAttr:
		// Check if attribute on player matches the pattern
		if b.Thing < 0 {
			return false
		}
		pattern := b.StrVal
		// Check player's attribute
		text := g.GetAttrText(player, b.Thing)
		if wildMatchCI(pattern, text) {
			return true
		}
		// Check inventory items
		for _, next := range g.DB.SafeContents(player) {
			iText := g.GetAttrText(next, b.Thing)
			if wildMatchCI(pattern, iText) {
				return true
			}
		}
		return false

	case gamedb.BoolEval:
		// Evaluate attribute on 'from' as softcode, then compare result to pattern
		if b.Thing < 0 {
			return false
		}
		attrText := g.GetAttrText(from, b.Thing)
		if attrText == "" {
			return false
		}
		// Evaluate the attribute as softcode with player as enactor
		ctx := MakeEvalContextForObj(g, from, player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		result := ctx.Exec(attrText, eval.EvFCheck|eval.EvEval, nil)
		return wildMatchCI(b.StrVal, result)

	case gamedb.BoolIndir:
		// Indirect lock: fetch LOCK attr from referenced object and evaluate it
		if b.Sub1 == nil || b.Sub1.Type != gamedb.BoolConst || b.Sub1.Thing < 0 {
			return false
		}
		target := gamedb.DBRef(b.Sub1.Thing)
		lockText := g.GetAttrText(target, aLock)
		if lockText == "" {
			// Also check Object.Lock for legacy header-based locks
			if tObj, ok := g.DB.Objects[target]; ok && tObj.Lock != nil {
				return EvalBoolExp(g, player, target, from, tObj.Lock, depth+1)
			}
			return true // no lock = pass
		}
		parsed := ParseBoolExp(g, player, lockText)
		return EvalBoolExp(g, player, target, from, parsed, depth+1)

	case gamedb.BoolCarry:
		// Player must CARRY the specified object or match attr on contents
		if b.Sub1 == nil {
			return false
		}
		if b.Sub1.Type == gamedb.BoolConst {
			return playerCarries(g, player, gamedb.DBRef(b.Sub1.Thing))
		}
		if b.Sub1.Type == gamedb.BoolAttr {
			if b.Sub1.Thing < 0 {
				return false
			}
			// Check ONLY contents, not the player
			for _, next := range g.DB.SafeContents(player) {
				iText := g.GetAttrText(next, b.Sub1.Thing)
				if wildMatchCI(b.Sub1.StrVal, iText) {
					return true
				}
			}
			return false
		}
		return false

	case gamedb.BoolIs:
		// Player must BE the specified object (exact identity) or match attr on player
		if b.Sub1 == nil {
			return false
		}
		if b.Sub1.Type == gamedb.BoolConst {
			return player == gamedb.DBRef(b.Sub1.Thing)
		}
		if b.Sub1.Type == gamedb.BoolAttr {
			if b.Sub1.Thing < 0 {
				return false
			}
			text := g.GetAttrText(player, b.Sub1.Thing)
			return wildMatchCI(b.Sub1.StrVal, text)
		}
		return false

	case gamedb.BoolOwner:
		// Player's owner must match the referenced object's owner
		if b.Sub1 == nil || b.Sub1.Type != gamedb.BoolConst {
			return false
		}
		target := gamedb.DBRef(b.Sub1.Thing)
		pObj, ok1 := g.DB.Objects[player]
		tObj, ok2 := g.DB.Objects[target]
		if !ok1 || !ok2 {
			return false
		}
		return pObj.Owner == tObj.Owner
	}

	return false
}

// playerCarries returns true if player has target in their contents chain.
func playerCarries(g *Game, player, target gamedb.DBRef) bool {
	for _, next := range g.DB.SafeContents(player) {
		if next == target {
			return true
		}
	}
	return false
}

// UnparseBoolExp converts a BoolExp tree back to a human-readable lock string.
func UnparseBoolExp(g *Game, b *gamedb.BoolExp) string {
	if b == nil {
		return ""
	}
	switch b.Type {
	case gamedb.BoolAnd:
		// Wrap left child in parens if it's an OR (lower precedence)
		left := UnparseBoolExp(g, b.Sub1)
		if b.Sub1 != nil && b.Sub1.Type == gamedb.BoolOr {
			left = "(" + left + ")"
		}
		return left + "&" + UnparseBoolExp(g, b.Sub2)
	case gamedb.BoolOr:
		return UnparseBoolExp(g, b.Sub1) + "|" + UnparseBoolExp(g, b.Sub2)
	case gamedb.BoolNot:
		return "!" + UnparseBoolExp(g, b.Sub1)
	case gamedb.BoolConst:
		ref := gamedb.DBRef(b.Thing)
		if ref == gamedb.Nothing {
			return "#-1"
		}
		name := g.ObjName(ref)
		if name != "" {
			return name + "(#" + strconv.Itoa(b.Thing) + ")"
		}
		return "#" + strconv.Itoa(b.Thing)
	case gamedb.BoolAttr:
		name := g.DB.GetAttrName(b.Thing)
		if name == "" {
			name = strconv.Itoa(b.Thing)
		}
		return name + ":" + b.StrVal
	case gamedb.BoolEval:
		name := g.DB.GetAttrName(b.Thing)
		if name == "" {
			name = strconv.Itoa(b.Thing)
		}
		return name + "/" + b.StrVal
	case gamedb.BoolIndir:
		return "@" + UnparseBoolExp(g, b.Sub1)
	case gamedb.BoolCarry:
		return "+" + UnparseBoolExp(g, b.Sub1)
	case gamedb.BoolIs:
		return "=" + UnparseBoolExp(g, b.Sub1)
	case gamedb.BoolOwner:
		return "$" + UnparseBoolExp(g, b.Sub1)
	}
	return "?"
}

// SerializeBoolExp converts a parsed BoolExp to a storable string using #dbref notation.
// Unlike UnparseBoolExp which displays names, this produces a form suitable for re-parsing.
func SerializeBoolExp(b *gamedb.BoolExp) string {
	return gamedb.SerializeBoolExp(b)
}

// wildMatchCI performs case-insensitive wildcard matching.
func wildMatchCI(pattern, str string) bool {
	return wildMatchSimple(strings.ToLower(pattern), strings.ToLower(str))
}

// ---------- High-Level Lock Check ----------

// CouldDoItStrict checks if player passes the lock without wizard bypass.
// Used for locks that should be absolute (e.g., leave locks on vehicles).
// Empty lock = unlocked (pass).
func CouldDoItStrict(g *Game, player, thing gamedb.DBRef, lockAttr int) bool {
	lockText := g.GetAttrText(thing, lockAttr)
	if lockText != "" {
		parsed := ParseBoolExp(g, player, lockText)
		return EvalBoolExp(g, player, thing, thing, parsed, 0)
	}
	// No lock = unlocked
	return true
}

// CouldDoIt checks if player passes the lock on thing for the given lock attribute.
// Wizards always pass (except against God). POW_PASS_LOCKS bypasses all locks.
// Empty lock = unlocked (pass).
func CouldDoIt(g *Game, player, thing gamedb.DBRef, lockAttr int) bool {
	// POW_PASS_LOCKS bypasses everything
	if PassLocks(g, player) {
		return true
	}

	// Wizards always pass — unless the lock owner is God
	if Wizard(g, player) {
		if !IsGod(g, thing) || IsGod(g, player) {
			return true
		}
	}

	// Check attribute-stored lock
	lockText := g.GetAttrText(thing, lockAttr)
	if lockText != "" {
		parsed := ParseBoolExp(g, player, lockText)
		return EvalBoolExp(g, player, thing, thing, parsed, 0)
	}

	// For default lock (attr 38), also check Object.Lock header-based lock
	if lockAttr == aLock {
		if tObj, ok := g.DB.Objects[thing]; ok && tObj.Lock != nil {
			return EvalBoolExp(g, player, thing, thing, tObj.Lock, 0)
		}
	}

	// No lock = unlocked
	return true
}

// HandleLockFailure sends failure messages and queues AFAIL action when a lock check fails.
func HandleLockFailure(g *Game, d *Descriptor, thing gamedb.DBRef, failAttr, oFailAttr, aFailAttr int, defaultMsg string) {
	// Show FAIL message to player (or default)
	failText := g.GetAttrText(thing, failAttr)
	if failText != "" {
		failText = evalExpr(g, d.Player, failText)
		d.Send(failText)
	} else {
		d.Send(defaultMsg)
	}

	// Show OFAIL message to room
	oFailText := g.GetAttrText(thing, oFailAttr)
	if oFailText != "" {
		oFailText = evalExpr(g, d.Player, oFailText)
		loc := g.PlayerLocation(d.Player)
		g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
			g.PlayerName(d.Player)+" "+oFailText)
	}

	// Queue AFAIL action
	g.QueueAttrAction(thing, d.Player, aFailAttr, nil)
}
