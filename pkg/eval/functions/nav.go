package functions

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Navigation functions for 3D grid-based movement and terrain systems.
// Covers flight (sled), ocean, and general zone navigation.
//
// Heading system: 32 compass points, 0=East, counterclockwise.
//   0=E, 4=NE, 8=N, 12=NW, 16=W, 20=SW, 24=S, 28=SE
//   Each step = 11.25 degrees.
//
// Grid system: 4 quadrants (NE, NW, SE, SW)
//   Letters AA-ZZ (W→E within each quadrant), 676 positions per quadrant
//   Numbers 000-999 (S→N within each quadrant), 1000 positions per quadrant
//   Altitude -1000 to +1000 (0 = ground level, negative = underground/underwater)
//   Address format: LL-NNN-QQ (e.g. EL-453-NE) or LL-NNN-QQ:ALT (e.g. EL-453-NE:500)
//
// Absolute coordinates: center of map is origin (0,0)
//   NE: x = letter_pos (0..675),  y = number (0..999)
//   NW: x = letter_pos - 676,     y = number
//   SE: x = letter_pos,           y = number - 1000
//   SW: x = letter_pos - 676,     y = number - 1000

const headingPoints = 32
const headingStep = 2 * math.Pi / float64(headingPoints) // 11.25 degrees in radians
const gridLetters = 676                                   // 26*26
const gridNumbers = 1000
const altMin = -1000
const altMax = 1000

// 32-point compass names, indexed by heading (0=E, counterclockwise)
var headingNames = [32]string{
	"E", "ENE", "ENE", "NE",
	"NE", "NNE", "NNE", "N",
	"N", "NNW", "NNW", "NW",
	"NW", "WNW", "WNW", "W",
	"W", "WSW", "WSW", "SW",
	"SW", "SSW", "SSW", "S",
	"S", "SSE", "SSE", "SE",
	"SE", "ESE", "ESE", "E",
}

// More precise 32 names for exact headings
var headingNamesExact = [32]string{
	"E", "EbN", "ENE", "NEbE",
	"NE", "NEbN", "NNE", "NbE",
	"N", "NbW", "NNW", "NWbN",
	"NW", "NWbW", "WNW", "WbN",
	"W", "WbS", "WSW", "SWbW",
	"SW", "SWbS", "SSW", "SbW",
	"S", "SbE", "SSE", "SEbS",
	"SE", "SEbE", "ESE", "EbS",
}

// fnHvec — convert heading (0-31) to unit direction vector.
// hvec(heading) → "dx dy"
func fnHvec(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	h := toInt(args[0])
	h = ((h % headingPoints) + headingPoints) % headingPoints // normalize to 0-31
	angle := float64(h) * headingStep
	dx := math.Cos(angle)
	dy := math.Sin(angle)
	// Clean up near-zero values
	if math.Abs(dx) < 1e-10 { dx = 0 }
	if math.Abs(dy) < 1e-10 { dy = 0 }
	writeFloat(buf, dx)
	buf.WriteByte(' ')
	writeFloat(buf, dy)
}

// fnHdelta — shortest turn between two headings.
// hdelta(h1, h2) → -16 to +16 (negative = turn left/counterclockwise, positive = clockwise)
func fnHdelta(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	h1 := ((toInt(args[0]) % headingPoints) + headingPoints) % headingPoints
	h2 := ((toInt(args[1]) % headingPoints) + headingPoints) % headingPoints
	delta := h2 - h1
	// Wrap to shortest path: -16 to +16
	if delta > headingPoints/2 {
		delta -= headingPoints
	} else if delta < -headingPoints/2 {
		delta += headingPoints
	}
	writeInt(buf, delta)
}

// fnHname — heading to compass name.
// hname(heading[, exact]) — if exact is 1, uses 32-point names; otherwise 16-point.
func fnHname(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	h := ((toInt(args[0]) % headingPoints) + headingPoints) % headingPoints
	if len(args) > 1 && toInt(args[1]) != 0 {
		buf.WriteString(headingNamesExact[h])
	} else {
		buf.WriteString(headingNames[h])
	}
}

// fnH2deg — heading to degrees.
// h2deg(heading) → degrees (0-360, 0=East counterclockwise)
func fnH2deg(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	h := toInt(args[0])
	deg := float64(h) * (360.0 / float64(headingPoints))
	writeFloat(buf, deg)
}

// fnDeg2h — degrees to nearest heading.
// deg2h(degrees) → heading 0-31
func fnDeg2h(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	deg := toFloat(args[0])
	// Normalize to 0-360
	deg = math.Mod(deg, 360)
	if deg < 0 { deg += 360 }
	h := int(math.Round(deg / (360.0 / float64(headingPoints))))
	h = h % headingPoints
	writeInt(buf, h)
}

// fnVec2h — direction vector to heading.
// vec2h(x, y) → heading 0-31
func fnVec2h(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	x := toFloat(args[0])
	y := toFloat(args[1])
	if x == 0 && y == 0 { buf.WriteString("0"); return }
	angle := math.Atan2(y, x)
	if angle < 0 { angle += 2 * math.Pi }
	h := int(math.Round(angle / headingStep))
	h = h % headingPoints
	writeInt(buf, h)
}

// --- Grid coordinate functions ---

// letterPos converts two-letter code (AA-ZZ) to position 0-675.
func letterPos(letters string) int {
	letters = strings.ToUpper(strings.TrimSpace(letters))
	if len(letters) != 2 { return -1 }
	a := int(letters[0] - 'A')
	b := int(letters[1] - 'A')
	if a < 0 || a > 25 || b < 0 || b > 25 { return -1 }
	return a*26 + b
}

// posToLetters converts position 0-675 to two-letter code.
func posToLetters(pos int) string {
	if pos < 0 || pos >= gridLetters { return "??" }
	a := pos / 26
	b := pos % 26
	return string([]byte{byte('A' + a), byte('A' + b)})
}

// gridToAbs converts grid location (letters, number, quadrant) to absolute x, y.
func gridToAbs(letters string, number int, quadrant string) (int, int, bool) {
	lp := letterPos(letters)
	if lp < 0 || number < 0 || number >= gridNumbers { return 0, 0, false }
	quadrant = strings.ToUpper(strings.TrimSpace(quadrant))
	var x, y int
	switch quadrant {
	case "NE":
		x = lp
		y = number
	case "NW":
		x = lp - gridLetters
		y = number
	case "SE":
		x = lp
		y = number - gridNumbers
	case "SW":
		x = lp - gridLetters
		y = number - gridNumbers
	default:
		return 0, 0, false
	}
	return x, y, true
}

// absToGridAlt converts absolute x, y, z to grid location string "LL-NNN-QQ:ALT".
func absToGridAlt(x, y, z int) string {
	base := absToGrid(x, y)
	return fmt.Sprintf("%s:%d", base, z)
}

// absToGrid converts absolute x, y to grid location string "LL-NNN-QQ".
func absToGrid(x, y int) string {
	var quad string
	var lp, num int
	if x >= 0 && y >= 0 {
		quad = "NE"
		lp = x
		num = y
	} else if x < 0 && y >= 0 {
		quad = "NW"
		lp = x + gridLetters
		num = y
	} else if x >= 0 && y < 0 {
		quad = "SE"
		lp = x
		num = y + gridNumbers
	} else {
		quad = "SW"
		lp = x + gridLetters
		num = y + gridNumbers
	}
	// Clamp
	if lp < 0 { lp = 0 }
	if lp >= gridLetters { lp = gridLetters - 1 }
	if num < 0 { num = 0 }
	if num >= gridNumbers { num = gridNumbers - 1 }
	return fmt.Sprintf("%s-%d-%s", posToLetters(lp), num, quad)
}

// parseGridLoc parses "LL-NNN-QQ" or "LL NNN QQ" format into absolute x, y.
func parseGridLoc(s string) (int, int, bool) {
	x, y, _, ok := parseGridLocFull(s)
	return x, y, ok
}

// parseGridLocFull parses "LL-NNN-QQ[:ALT]" or "LL NNN QQ [ALT]" into x, y, z.
// If altitude is omitted, z defaults to 0.
func parseGridLocFull(s string) (int, int, int, bool) {
	s = strings.TrimSpace(s)
	z := 0

	// Check for :ALT suffix
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		z = toInt(s[idx+1:])
		s = s[:idx]
	}

	// Try dash-delimited first: EL-453-NE
	parts := strings.Split(s, "-")
	if len(parts) == 3 {
		num := toInt(parts[1])
		x, y, ok := gridToAbs(parts[0], num, parts[2])
		return x, y, z, ok
	}
	// Try space-delimited: EL 453 NE [alt]
	parts = strings.Fields(s)
	if len(parts) >= 3 {
		num := toInt(parts[1])
		x, y, ok := gridToAbs(parts[0], num, parts[2])
		if ok && len(parts) >= 4 {
			z = toInt(parts[3])
		}
		return x, y, z, ok
	}
	return 0, 0, 0, false
}

// fnGridabs — convert grid location to absolute coordinates.
// gridabs(letters, number, quadrant) → "x y"
// gridabs(LL-NNN-QQ) → "x y" (single-arg form)
func fnGridabs(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	var x, y int
	var ok bool
	if len(args) >= 3 {
		num := toInt(args[1])
		x, y, ok = gridToAbs(args[0], num, args[2])
	} else {
		x, y, ok = parseGridLoc(args[0])
	}
	if !ok {
		buf.WriteString("#-1 INVALID GRID LOCATION")
		return
	}
	writeInt(buf, x)
	buf.WriteByte(' ')
	writeInt(buf, y)
}

// fnAbsgrid — convert absolute coordinates to grid location string.
// absgrid(x, y) → "LL-NNN-QQ"
func fnAbsgrid(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	x := toInt(args[0])
	y := toInt(args[1])
	buf.WriteString(absToGrid(x, y))
}

// fnGriddist — distance between two grid locations.
// griddist(loc1, loc2) → distance (2D, ignoring altitude)
func fnGriddist(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	x1, y1, ok1 := parseGridLoc(args[0])
	x2, y2, ok2 := parseGridLoc(args[1])
	if !ok1 || !ok2 {
		buf.WriteString("#-1 INVALID GRID LOCATION")
		return
	}
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	writeFloat(buf, math.Sqrt(dx*dx+dy*dy))
}

// fnGridcourse — calculate heading and distance from one grid loc to another.
// gridcourse(from, to) → "heading distance"
func fnGridcourse(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	x1, y1, ok1 := parseGridLoc(args[0])
	x2, y2, ok2 := parseGridLoc(args[1])
	if !ok1 || !ok2 {
		buf.WriteString("#-1 INVALID GRID LOCATION")
		return
	}
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 0.001 {
		buf.WriteString("0 0")
		return
	}
	angle := math.Atan2(dy, dx)
	if angle < 0 { angle += 2 * math.Pi }
	h := int(math.Round(angle / headingStep))
	h = h % headingPoints
	writeInt(buf, h)
	buf.WriteByte(' ')
	writeFloat(buf, dist)
}

// fnGridnav — project a new position given current pos, heading, speed, climb, and drift.
// gridnav(x y z, heading, speed[, climb[, drift]]) → "x y z"
// drift is maximum random perturbation per axis per tick.
func fnGridnav(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	pos := parseVector(args[0])
	if len(pos) < 2 { return }
	h := toInt(args[1])
	speed := toFloat(args[2])
	climb := 0.0
	if len(args) > 3 { climb = toFloat(args[3]) }
	drift := 0.0
	if len(args) > 4 { drift = toFloat(args[4]) }

	h = ((h % headingPoints) + headingPoints) % headingPoints
	angle := float64(h) * headingStep
	dx := math.Cos(angle) * speed
	dy := math.Sin(angle) * speed

	newX := pos[0] + dx
	newY := pos[1] + dy
	newZ := 0.0
	if len(pos) >= 3 {
		newZ = pos[2] + climb
	}

	// Apply drift: random perturbation in [-drift, +drift] per axis
	if drift > 0 {
		newX += (rand.Float64()*2 - 1) * drift
		newY += (rand.Float64()*2 - 1) * drift
		newZ += (rand.Float64()*2 - 1) * drift
	}

	// Clamp altitude to valid range
	if newZ < float64(altMin) { newZ = float64(altMin) }
	if newZ > float64(altMax) { newZ = float64(altMax) }

	writeFloat(buf, newX)
	buf.WriteByte(' ')
	writeFloat(buf, newY)
	buf.WriteByte(' ')
	writeFloat(buf, newZ)
}

// --- Random vector / drift functions ---

// fnVrand — generate a random direction vector with magnitude between 0 and max.
// vrand(max_magnitude[, dimensions]) → "x y z"
// The direction is uniformly random; magnitude is uniform [0, max].
// Default dimensions = 3.
func fnVrand(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	maxMag := toFloat(args[0])
	dims := 3
	if len(args) > 1 {
		d := toInt(args[1])
		if d >= 1 && d <= 10 { dims = d }
	}

	// Generate random unit direction via normal distribution (uniform on sphere)
	v := make([]float64, dims)
	norm := 0.0
	for i := range v {
		g := rand.NormFloat64()
		v[i] = g
		norm += g * g
	}
	norm = math.Sqrt(norm)
	if norm < 1e-15 {
		// Degenerate case: just return zeros
		writeVector(buf, v)
		return
	}

	// Scale to random magnitude [0, max]
	mag := rand.Float64() * maxMag
	for i := range v {
		v[i] = v[i] / norm * mag
	}
	writeVector(buf, v)
}

// fnVrandc — per-component random vector in [-max, +max] for each component.
// vrandc(max_x max_y max_z) → "dx dy dz"
// Each component is independently randomized in [-max_i, +max_i].
// This is useful for rectangular drift zones (e.g., different drift on altitude vs XY).
func fnVrandc(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	maxVec := parseVector(args[0])
	if len(maxVec) == 0 { return }
	r := make([]float64, len(maxVec))
	for i, m := range maxVec {
		r[i] = (rand.Float64()*2 - 1) * m
	}
	writeVector(buf, r)
}

// fnDrift — apply random drift to a position vector.
// drift(position, max_drift) → "x y z"
// max_drift can be a single number (uniform per axis) or a vector (per-component max).
func fnDrift(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	pos := parseVector(args[0])
	if len(pos) == 0 { return }

	// Check if max_drift is a single scalar or a vector
	driftSpec := strings.TrimSpace(args[1])
	driftVec := parseVector(driftSpec)

	r := make([]float64, len(pos))
	copy(r, pos)

	if len(driftVec) == 1 {
		// Uniform drift: same max for all axes
		d := driftVec[0]
		for i := range r {
			r[i] += (rand.Float64()*2 - 1) * d
		}
	} else if len(driftVec) >= len(pos) {
		// Per-component drift
		for i := range r {
			r[i] += (rand.Float64()*2 - 1) * driftVec[i]
		}
	} else {
		// Partial: drift what we can, leave rest unchanged
		for i := range driftVec {
			if i < len(r) {
				r[i] += (rand.Float64()*2 - 1) * driftVec[i]
			}
		}
	}

	writeVector(buf, r)
}

// --- Multi-object tactical navigation functions ---

// headingToVec converts heading 0-31 + speed to a velocity vector (dx, dy).
func headingToVec(h int, speed float64) (float64, float64) {
	h = ((h % headingPoints) + headingPoints) % headingPoints
	angle := float64(h) * headingStep
	return math.Cos(angle) * speed, math.Sin(angle) * speed
}

// fnBearing — heading from position 1 to position 2.
// bearing(x1 y1 [z1], x2 y2 [z2]) → heading 0-31
// Returns the heading obj1 would need to fly to face obj2 (2D, ignores Z).
func fnBearing(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	a := parseVector(args[0])
	b := parseVector(args[1])
	if len(a) < 2 || len(b) < 2 { buf.WriteString("0"); return }
	dx := b[0] - a[0]
	dy := b[1] - a[1]
	if dx == 0 && dy == 0 { buf.WriteString("0"); return }
	angle := math.Atan2(dy, dx)
	if angle < 0 { angle += 2 * math.Pi }
	h := int(math.Round(angle / headingStep))
	h = h % headingPoints
	writeInt(buf, h)
}

// fnPitch — vertical angle (climb/dive) from position 1 to position 2 in degrees.
// pitch(x1 y1 z1, x2 y2 z2) → degrees (-90 to +90, positive = climbing)
func fnPitch(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	a := parseVector(args[0])
	b := parseVector(args[1])
	if len(a) < 3 || len(b) < 3 { buf.WriteString("0"); return }
	dx := b[0] - a[0]
	dy := b[1] - a[1]
	dz := b[2] - a[2]
	horiz := math.Sqrt(dx*dx + dy*dy)
	if horiz < 0.001 && math.Abs(dz) < 0.001 { buf.WriteString("0"); return }
	pitch := math.Atan2(dz, horiz) * (180.0 / math.Pi)
	writeFloat(buf, pitch)
}

// fnClosing — closing rate between two moving objects.
// closing(pos1, heading1, speed1, pos2, heading2, speed2) → rate
// Positive = getting closer, negative = separating.
// Rate is in distance units per tick.
func fnClosing(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 6 { buf.WriteString("0"); return }
	p1 := parseVector(args[0])
	h1 := toInt(args[1])
	s1 := toFloat(args[2])
	p2 := parseVector(args[3])
	h2 := toInt(args[4])
	s2 := toFloat(args[5])
	if len(p1) < 2 || len(p2) < 2 { buf.WriteString("0"); return }

	// Current distance
	dx := p2[0] - p1[0]
	dy := p2[1] - p1[1]
	distNow := math.Sqrt(dx*dx + dy*dy)
	if distNow < 0.001 { buf.WriteString("0"); return }

	// Velocity vectors
	vx1, vy1 := headingToVec(h1, s1)
	vx2, vy2 := headingToVec(h2, s2)

	// Positions after 1 tick
	nx1 := p1[0] + vx1
	ny1 := p1[1] + vy1
	nx2 := p2[0] + vx2
	ny2 := p2[1] + vy2

	ndx := nx2 - nx1
	ndy := ny2 - ny1
	distNext := math.Sqrt(ndx*ndx + ndy*ndy)

	// Closing rate: positive means getting closer
	writeFloat(buf, distNow-distNext)
}

// fnRelvel — relative velocity vector between two objects.
// relvel(heading1, speed1, heading2, speed2) → "dx dy"
// Returns velocity of obj2 relative to obj1 (from obj1's perspective).
func fnRelvel(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 4 { return }
	vx1, vy1 := headingToVec(toInt(args[0]), toFloat(args[1]))
	vx2, vy2 := headingToVec(toInt(args[2]), toFloat(args[3]))
	writeFloat(buf, vx2-vx1)
	buf.WriteByte(' ')
	writeFloat(buf, vy2-vy1)
}

// fnEta — estimated ticks to reach target at current heading and speed.
// eta(pos1, heading, speed, pos2) → ticks (or -1 if moving away / stopped)
// This is straight-line ETA assuming target is stationary.
func fnEta(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 4 { buf.WriteString("-1"); return }
	p1 := parseVector(args[0])
	h := toInt(args[1])
	speed := toFloat(args[2])
	p2 := parseVector(args[3])
	if len(p1) < 2 || len(p2) < 2 || speed <= 0 {
		buf.WriteString("-1"); return
	}

	dx := p2[0] - p1[0]
	dy := p2[1] - p1[1]
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 0.001 { buf.WriteString("0"); return }

	// Velocity vector
	vx, vy := headingToVec(h, speed)

	// Project distance onto velocity direction (how much of our speed goes toward target)
	closingSpeed := (dx*vx + dy*vy) / dist
	if closingSpeed <= 0 {
		buf.WriteString("-1") // Moving away or perpendicular
		return
	}

	ticks := dist / closingSpeed
	writeFloat(buf, ticks)
}

// fnIntercept — calculate heading for obj1 to intercept moving obj2.
// intercept(pos1, speed1, pos2, heading2, speed2) → heading (0-31) or -1 if impossible
// Uses binary search on time to find the heading obj1 should fly.
func fnIntercept(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 5 { buf.WriteString("-1"); return }
	p1 := parseVector(args[0])
	s1 := toFloat(args[1])
	p2 := parseVector(args[2])
	h2 := toInt(args[3])
	s2 := toFloat(args[4])
	if len(p1) < 2 || len(p2) < 2 || s1 <= 0 {
		buf.WriteString("-1"); return
	}

	// Target velocity
	vx2, vy2 := headingToVec(h2, s2)

	// Vector from p1 to p2
	dx := p2[0] - p1[0]
	dy := p2[1] - p1[1]
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 0.001 { buf.WriteString("0"); return }

	// Binary search for intercept time
	// At time t: target at p2 + t*(vx2,vy2), we need dist(p1, target_t) == s1*t
	lo, hi := 0.0, dist/s1*10
	if hi < 10 { hi = 10 }

	bestT := (lo + hi) / 2
	for iter := 0; iter < 50; iter++ {
		t := (lo + hi) / 2
		tx := p2[0] + vx2*t
		ty := p2[1] + vy2*t
		ix := tx - p1[0]
		iy := ty - p1[1]
		needed := math.Sqrt(ix*ix + iy*iy)
		canCover := s1 * t
		if math.Abs(needed-canCover) < 0.5 {
			bestT = t
			break
		}
		if needed > canCover {
			lo = t
		} else {
			hi = t
		}
		bestT = t
	}

	// Calculate intercept point and heading to it
	tx := p2[0] + vx2*bestT
	ty := p2[1] + vy2*bestT
	ix := tx - p1[0]
	iy := ty - p1[1]
	if math.Abs(ix) < 0.001 && math.Abs(iy) < 0.001 {
		buf.WriteString("0"); return
	}

	angle := math.Atan2(iy, ix)
	if angle < 0 { angle += 2 * math.Pi }
	h := int(math.Round(angle / headingStep))
	h = h % headingPoints
	writeInt(buf, h)
}

// --- GPS / Topography / Instance functions ---

// fnAltclamp — clamp a value to the valid altitude range.
// altclamp(z) → clamped z (-1000 to 1000)
func fnAltclamp(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	z := toInt(args[0])
	if z < altMin { z = altMin }
	if z > altMax { z = altMax }
	writeInt(buf, z)
}

// fnGridlocfull — convert absolute x, y, z to full grid location with altitude.
// gridlocfull(x, y, z) → "LL-NNN-QQ:ALT"
func fnGridlocfull(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	x := toInt(args[0])
	y := toInt(args[1])
	z := toInt(args[2])
	buf.WriteString(absToGridAlt(x, y, z))
}

// fnGridparsefull — parse grid location with optional altitude to x y z.
// gridparsefull(LL-NNN-QQ[:ALT]) → "x y z"
func fnGridparsefull(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	x, y, z, ok := parseGridLocFull(args[0])
	if !ok {
		buf.WriteString("#-1 INVALID GRID LOCATION")
		return
	}
	writeInt(buf, x)
	buf.WriteByte(' ')
	writeInt(buf, y)
	buf.WriteByte(' ')
	writeInt(buf, z)
}

// fnGps — full GPS string for a position: grid address, altitude, and compass heading.
// gps(x y z[, heading]) → "LL-NNN-QQ ALT:nnn HDG:compass"
func fnGps(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	pos := parseVector(args[0])
	if len(pos) < 2 { return }
	x := int(math.Round(pos[0]))
	y := int(math.Round(pos[1]))
	z := 0
	if len(pos) >= 3 { z = int(math.Round(pos[2])) }
	grid := absToGrid(x, y)
	buf.WriteString(grid)
	buf.WriteString(fmt.Sprintf(" ALT:%d", z))
	if len(args) > 1 {
		h := ((toInt(args[1]) % headingPoints) + headingPoints) % headingPoints
		buf.WriteString(fmt.Sprintf(" HDG:%s", headingNames[h]))
	}
}

// fnGriddist3d — 3D distance between two grid locations (with altitude).
// griddist3d(loc1[:alt1], loc2[:alt2]) → distance
func fnGriddist3d(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	x1, y1, z1, ok1 := parseGridLocFull(args[0])
	x2, y2, z2, ok2 := parseGridLocFull(args[1])
	if !ok1 || !ok2 {
		buf.WriteString("#-1 INVALID GRID LOCATION")
		return
	}
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	dz := float64(z2 - z1)
	writeFloat(buf, math.Sqrt(dx*dx+dy*dy+dz*dz))
}

// fnMapinstance — construct an instanced grid location key.
// mapinstance(instance_id, LL-NNN-QQ[:ALT]) → "instance_id|LL-NNN-QQ[:ALT]"
// mapinstance(instance_id, x, y[, z]) → "instance_id|LL-NNN-QQ[:ALT]"
// Used for multi-planet/multi-map systems where each planet has its own grid.
func fnMapinstance(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	instanceID := strings.TrimSpace(args[0])
	if instanceID == "" { return }

	if len(args) >= 4 {
		// Numeric form: mapinstance(id, x, y[, z])
		x := toInt(args[1])
		y := toInt(args[2])
		z := 0
		if len(args) >= 5 { z = toInt(args[3]) }
		if z != 0 {
			buf.WriteString(fmt.Sprintf("%s|%s", instanceID, absToGridAlt(x, y, z)))
		} else {
			buf.WriteString(fmt.Sprintf("%s|%s", instanceID, absToGrid(x, y)))
		}
	} else {
		// Grid string form: mapinstance(id, LL-NNN-QQ[:ALT])
		buf.WriteString(fmt.Sprintf("%s|%s", instanceID, strings.TrimSpace(args[1])))
	}
}

// fnMapparse — split an instanced grid location key into instance and location.
// mapparse(key, component) → value
// component: "instance" → instance ID, "loc" → grid location, "x"/"y"/"z" → coordinate
func fnMapparse(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	key := strings.TrimSpace(args[0])
	component := strings.ToLower(strings.TrimSpace(args[1]))

	// Split on pipe
	pipeIdx := strings.Index(key, "|")
	var instanceID, locStr string
	if pipeIdx >= 0 {
		instanceID = key[:pipeIdx]
		locStr = key[pipeIdx+1:]
	} else {
		// No instance prefix — treat whole thing as location
		locStr = key
	}

	switch component {
	case "instance", "id":
		buf.WriteString(instanceID)
	case "loc", "location":
		buf.WriteString(locStr)
	case "x":
		x, _, _, ok := parseGridLocFull(locStr)
		if ok { writeInt(buf, x) } else { buf.WriteString("0") }
	case "y":
		_, y, _, ok := parseGridLocFull(locStr)
		if ok { writeInt(buf, y) } else { buf.WriteString("0") }
	case "z", "alt":
		_, _, z, ok := parseGridLocFull(locStr)
		if ok { writeInt(buf, z) } else { buf.WriteString("0") }
	default:
		buf.WriteString("#-1 INVALID COMPONENT")
	}
}

// --- Point of Interest (POI) functions ---
// A POI is stored as an attribute value in the format:
//   x y z height|instance|name|tags
// Example: "100 200 50 25|terra|Crystal Tower|landmark quest"
// - x y z: position on the grid
// - height: vertical extent (POI occupies z to z+height)
// - instance: map/planet instance ID (empty = default map)
// - name: display name
// - tags: space-separated tags for filtering (e.g., "city quest shop")

// fnPoiformat — format a POI attribute value.
// poiformat(x, y, z, height, instance, name[, tags]) → "x y z height|instance|name|tags"
func fnPoiformat(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 6 { return }
	x := toInt(args[0])
	y := toInt(args[1])
	z := toInt(args[2])
	height := toInt(args[3])
	instance := strings.TrimSpace(args[4])
	name := strings.TrimSpace(args[5])
	tags := ""
	if len(args) >= 7 { tags = strings.TrimSpace(args[6]) }
	buf.WriteString(fmt.Sprintf("%d %d %d %d|%s|%s|%s", x, y, z, height, instance, name, tags))
}

// fnPoiparse — extract a component from a POI attribute value.
// poiparse(poi_value, component) → value
// Components: x, y, z, height, instance, name, tags, grid, gps, pos
func fnPoiparse(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	poi := strings.TrimSpace(args[0])
	component := strings.ToLower(strings.TrimSpace(args[1]))

	// Split on pipes: "x y z h|instance|name|tags"
	parts := strings.SplitN(poi, "|", 4)
	if len(parts) < 1 { return }

	// Parse the coordinate part: "x y z height"
	coords := parseVector(parts[0])
	x, y, z, h := 0, 0, 0, 0
	if len(coords) >= 1 { x = int(coords[0]) }
	if len(coords) >= 2 { y = int(coords[1]) }
	if len(coords) >= 3 { z = int(coords[2]) }
	if len(coords) >= 4 { h = int(coords[3]) }

	instance := ""
	if len(parts) >= 2 { instance = parts[1] }
	name := ""
	if len(parts) >= 3 { name = parts[2] }
	tags := ""
	if len(parts) >= 4 { tags = parts[3] }

	switch component {
	case "x":
		writeInt(buf, x)
	case "y":
		writeInt(buf, y)
	case "z", "alt":
		writeInt(buf, z)
	case "height", "h":
		writeInt(buf, h)
	case "instance", "id":
		buf.WriteString(instance)
	case "name":
		buf.WriteString(name)
	case "tags":
		buf.WriteString(tags)
	case "pos":
		// "x y z"
		writeInt(buf, x)
		buf.WriteByte(' ')
		writeInt(buf, y)
		buf.WriteByte(' ')
		writeInt(buf, z)
	case "grid":
		buf.WriteString(absToGrid(x, y))
	case "gps":
		buf.WriteString(absToGridAlt(x, y, z))
	default:
		buf.WriteString("#-1 INVALID COMPONENT")
	}
}

// fnPoiinrange — check if a position is within range of a POI (4D: x, y, z + height).
// poiinrange(poi_value, x y z, radius) → 1 or 0
// Checks horizontal distance <= radius AND altitude overlaps POI's z to z+height.
func fnPoiinrange(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("0"); return }
	poi := strings.TrimSpace(args[0])
	parts := strings.SplitN(poi, "|", 4)
	if len(parts) < 1 { buf.WriteString("0"); return }

	coords := parseVector(parts[0])
	if len(coords) < 3 { buf.WriteString("0"); return }
	px, py, pz := coords[0], coords[1], coords[2]
	ph := 0.0
	if len(coords) >= 4 { ph = coords[3] }

	pos := parseVector(args[1])
	if len(pos) < 3 { buf.WriteString("0"); return }
	radius := toFloat(args[2])

	// Horizontal distance
	dx := pos[0] - px
	dy := pos[1] - py
	hDist := math.Sqrt(dx*dx + dy*dy)

	// Vertical overlap: POI spans pz to pz+ph, position is at pos[2]
	// Distance from pos[2] to nearest point in [pz, pz+ph]
	vDist := 0.0
	if pos[2] < pz {
		vDist = pz - pos[2]
	} else if pos[2] > pz+ph {
		vDist = pos[2] - (pz + ph)
	}

	totalDist := math.Sqrt(hDist*hDist + vDist*vDist)
	if totalDist <= radius {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// fnPoidist — distance from a position to the nearest point of a POI (4D).
// poidist(poi_value, x y z) → distance
func fnPoidist(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	poi := strings.TrimSpace(args[0])
	parts := strings.SplitN(poi, "|", 4)
	if len(parts) < 1 { buf.WriteString("0"); return }

	coords := parseVector(parts[0])
	if len(coords) < 3 { buf.WriteString("0"); return }
	px, py, pz := coords[0], coords[1], coords[2]
	ph := 0.0
	if len(coords) >= 4 { ph = coords[3] }

	pos := parseVector(args[1])
	if len(pos) < 3 { buf.WriteString("0"); return }

	dx := pos[0] - px
	dy := pos[1] - py
	hDist := math.Sqrt(dx*dx + dy*dy)

	vDist := 0.0
	if pos[2] < pz {
		vDist = pz - pos[2]
	} else if pos[2] > pz+ph {
		vDist = pos[2] - (pz + ph)
	}

	writeFloat(buf, math.Sqrt(hDist*hDist+vDist*vDist))
}

// fnPoibearing — heading from a position to a POI.
// poibearing(poi_value, x y z) → heading 0-31
func fnPoibearing(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	poi := strings.TrimSpace(args[0])
	parts := strings.SplitN(poi, "|", 4)
	if len(parts) < 1 { buf.WriteString("0"); return }

	coords := parseVector(parts[0])
	if len(coords) < 2 { buf.WriteString("0"); return }

	pos := parseVector(args[1])
	if len(pos) < 2 { buf.WriteString("0"); return }

	dx := coords[0] - pos[0]
	dy := coords[1] - pos[1]
	if dx == 0 && dy == 0 { buf.WriteString("0"); return }
	angle := math.Atan2(dy, dx)
	if angle < 0 { angle += 2 * math.Pi }
	h := int(math.Round(angle / headingStep))
	h = h % headingPoints
	writeInt(buf, h)
}

// --- Topology / Terrain functions ---
//
// topology(zone_dbref, x, y) → elevation (float)
//
// Returns the terrain elevation at (x, y) within the given zone.
// Positive = above sea level (land), negative = below sea level (water depth).
// Zero = sea level (shoreline). Deterministic: same inputs always return same output.
//
// The zone object defines terrain via attributes:
//   TOPO_SEED       - integer seed for noise layer (makes each zone unique)
//   TOPO_BASE       - baseline elevation (e.g., -20 for ocean, 100 for mountains)
//   TOPO_GRADIENT   - "axis rate" e.g., "x -0.15" = -0.15 per unit eastward
//   TOPO_NOISE_AMP  - amplitude of noise layer (roughness), default 2.0
//   TOPO_NOISE_FREQ - frequency of noise layer (feature size), default 0.05
//   TOPO_FEATURES   - count of terrain features
//   TOPO_F_1..N     - feature definitions (see below)
//
// Feature types:
//   seamount|cx cy|radius|falloff|peak_elev
//     Circular bump. Cosine falloff from peak at center to 0 at radius.
//   trench|x1 y1 x2 y2|width|floor_elev
//     Linear depression along a line segment. Cosine cross-section.
//   island|cx cy|radius|peak_elev
//     Like seamount but peak is positive (land). Creates beach ring at sea level.
//   ridge|x1 y1 x2 y2|width|peak_elev
//     Linear rise along a line segment. Cosine cross-section.
//   shelf|x_start x_end|slope
//     Gradual slope change between two X boundaries.
//   basin|cx cy|radius|floor_elev
//     Circular depression (inverse seamount).

// topoZone stores parsed terrain data per zone to avoid re-parsing attrs every call.
type topoZone struct {
	seed      int64
	base      float64
	gradAxis  byte    // 'x' or 'y'
	gradRate  float64
	noiseAmp  float64
	noiseFreq float64
	features  []topoFeature

	// Current/flow fields
	currBaseX   float64 // prevailing current vector (dx per tick)
	currBaseY   float64
	currDeflect float64 // topology deflection strength (default 1.0)
	currTidal   float64 // tidal oscillation amplitude
	currTidalHd float64 // tidal heading (radians)
}

type topoFeatureType int

const (
	topoSeamount topoFeatureType = iota
	topoTrench
	topoIsland
	topoRidge
	topoShelf
	topoBasin
)

type topoFeature struct {
	ftype    topoFeatureType
	cx, cy   float64 // center or line start
	x2, y2   float64 // line end (for trench/ridge)
	radius   float64
	width    float64 // for linear features
	slope    float64 // for shelf
	peakElev float64 // peak or floor elevation
}

var topoCache = make(map[gamedb.DBRef]*topoZone)

// topoInvalidate clears cached topology for a zone (call when attrs change).
func topoInvalidate(zone gamedb.DBRef) {
	delete(topoCache, zone)
}

// topoParse reads topology attrs from a zone object and caches the result.
func topoParse(ctx *eval.EvalContext, zone gamedb.DBRef) *topoZone {
	if cached, ok := topoCache[zone]; ok {
		return cached
	}

	tz := &topoZone{
		base:      -20.0,
		noiseAmp:  2.0,
		noiseFreq: 0.05,
	}

	// TOPO_SEED
	if s := getAttrByName(ctx, zone, "TOPO_SEED"); s != "" {
		tz.seed = int64(toInt(s))
	}

	// TOPO_BASE
	if s := getAttrByName(ctx, zone, "TOPO_BASE"); s != "" {
		tz.base = toFloat(s)
	}

	// TOPO_GRADIENT: "axis rate" e.g., "x -0.15"
	if s := getAttrByName(ctx, zone, "TOPO_GRADIENT"); s != "" {
		parts := strings.Fields(s)
		if len(parts) >= 2 {
			axis := strings.ToLower(parts[0])
			if len(axis) > 0 {
				tz.gradAxis = axis[0]
			}
			tz.gradRate = toFloat(parts[1])
		}
	}

	// TOPO_NOISE_AMP
	if s := getAttrByName(ctx, zone, "TOPO_NOISE_AMP"); s != "" {
		tz.noiseAmp = toFloat(s)
	}

	// TOPO_NOISE_FREQ
	if s := getAttrByName(ctx, zone, "TOPO_NOISE_FREQ"); s != "" {
		tz.noiseFreq = toFloat(s)
	}

	// TOPO_FEATURES count
	featureCount := 0
	if s := getAttrByName(ctx, zone, "TOPO_FEATURES"); s != "" {
		featureCount = toInt(s)
	}

	// Parse each TOPO_F_N
	for i := 1; i <= featureCount; i++ {
		attrName := fmt.Sprintf("TOPO_F_%d", i)
		s := getAttrByName(ctx, zone, attrName)
		if s == "" {
			continue
		}
		f, ok := parseTopoFeature(s)
		if ok {
			tz.features = append(tz.features, f)
		}
	}

	// CURRENT_BASE: "heading strength" — prevailing current/wind
	// heading in compass degrees (0=N, 90=E, 180=S, 270=W), strength in units/tick
	tz.currDeflect = 1.0 // default deflection factor
	if s := getAttrByName(ctx, zone, "CURRENT_BASE"); s != "" {
		parts := strings.Fields(s)
		if len(parts) >= 2 {
			hdgDeg := toFloat(parts[0])
			strength := toFloat(parts[1])
			// Convert compass heading to dx,dy vector
			// 0=N(+y), 90=E(+x), 180=S(-y), 270=W(-x)
			hdgRad := hdgDeg * math.Pi / 180.0
			tz.currBaseX = math.Sin(hdgRad) * strength
			tz.currBaseY = math.Cos(hdgRad) * strength
		}
	}

	// CURRENT_DEFLECT: factor for topology deflection (default 1.0, 0=no deflection)
	if s := getAttrByName(ctx, zone, "CURRENT_DEFLECT"); s != "" {
		tz.currDeflect = toFloat(s)
	}

	// CURRENT_TIDAL: "heading strength" — tidal current component
	// Oscillates with external tide phase passed to current()
	if s := getAttrByName(ctx, zone, "CURRENT_TIDAL"); s != "" {
		parts := strings.Fields(s)
		if len(parts) >= 2 {
			hdgDeg := toFloat(parts[0])
			tz.currTidalHd = hdgDeg * math.Pi / 180.0
			tz.currTidal = toFloat(parts[1])
		}
	}

	topoCache[zone] = tz
	return tz
}

// parseTopoFeature parses a feature definition string.
func parseTopoFeature(s string) (topoFeature, bool) {
	parts := strings.Split(s, "|")
	if len(parts) < 3 {
		return topoFeature{}, false
	}

	var f topoFeature
	ftype := strings.ToLower(strings.TrimSpace(parts[0]))

	switch ftype {
	case "seamount":
		// seamount|cx cy|radius|falloff|peak_elev
		if len(parts) < 5 {
			return f, false
		}
		center := parseVector(parts[1])
		if len(center) < 2 {
			return f, false
		}
		f.ftype = topoSeamount
		f.cx, f.cy = center[0], center[1]
		f.radius = toFloat(parts[2])
		// parts[3] is falloff (reserved for future use, cosine falloff is standard)
		f.peakElev = toFloat(parts[4])
		return f, true

	case "island":
		// island|cx cy|radius|peak_elev
		if len(parts) < 4 {
			return f, false
		}
		center := parseVector(parts[1])
		if len(center) < 2 {
			return f, false
		}
		f.ftype = topoIsland
		f.cx, f.cy = center[0], center[1]
		f.radius = toFloat(parts[2])
		f.peakElev = toFloat(parts[3])
		return f, true

	case "basin":
		// basin|cx cy|radius|floor_elev
		if len(parts) < 4 {
			return f, false
		}
		center := parseVector(parts[1])
		if len(center) < 2 {
			return f, false
		}
		f.ftype = topoBasin
		f.cx, f.cy = center[0], center[1]
		f.radius = toFloat(parts[2])
		f.peakElev = toFloat(parts[3])
		return f, true

	case "trench":
		// trench|x1 y1 x2 y2|width|floor_elev
		if len(parts) < 4 {
			return f, false
		}
		coords := parseVector(parts[1])
		if len(coords) < 4 {
			return f, false
		}
		f.ftype = topoTrench
		f.cx, f.cy = coords[0], coords[1]
		f.x2, f.y2 = coords[2], coords[3]
		f.width = toFloat(parts[2])
		f.peakElev = toFloat(parts[3])
		return f, true

	case "ridge":
		// ridge|x1 y1 x2 y2|width|peak_elev
		if len(parts) < 4 {
			return f, false
		}
		coords := parseVector(parts[1])
		if len(coords) < 4 {
			return f, false
		}
		f.ftype = topoRidge
		f.cx, f.cy = coords[0], coords[1]
		f.x2, f.y2 = coords[2], coords[3]
		f.width = toFloat(parts[2])
		f.peakElev = toFloat(parts[3])
		return f, true

	case "shelf":
		// shelf|x_start x_end|slope
		if len(parts) < 3 {
			return f, false
		}
		bounds := parseVector(parts[1])
		if len(bounds) < 2 {
			return f, false
		}
		f.ftype = topoShelf
		f.cx = bounds[0] // x_start
		f.x2 = bounds[1] // x_end
		f.slope = toFloat(parts[2])
		return f, true

	default:
		return f, false
	}
}

// topoElevation computes the elevation at (x, y) for a parsed zone.
func topoElevation(tz *topoZone, x, y float64) float64 {
	// Start with base elevation
	elev := tz.base

	// Apply gradient
	switch tz.gradAxis {
	case 'x':
		elev += x * tz.gradRate
	case 'y':
		elev += y * tz.gradRate
	}

	// Apply each feature
	for _, f := range tz.features {
		elev += topoFeatureContrib(f, x, y)
	}

	// Apply deterministic noise
	if tz.noiseAmp > 0 {
		elev += topoNoise(tz.seed, x*tz.noiseFreq, y*tz.noiseFreq) * tz.noiseAmp
	}

	return elev
}

// topoFeatureContrib returns the elevation contribution of a single feature at (x, y).
func topoFeatureContrib(f topoFeature, x, y float64) float64 {
	switch f.ftype {
	case topoSeamount, topoIsland:
		// Circular feature with cosine falloff
		dx := x - f.cx
		dy := y - f.cy
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist >= f.radius {
			return 0
		}
		t := dist / f.radius
		return f.peakElev * 0.5 * (1.0 + math.Cos(t*math.Pi))

	case topoBasin:
		// Circular depression with cosine falloff
		dx := x - f.cx
		dy := y - f.cy
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist >= f.radius {
			return 0
		}
		t := dist / f.radius
		return f.peakElev * 0.5 * (1.0 + math.Cos(t*math.Pi))

	case topoTrench:
		return topoLinearContrib(f.cx, f.cy, f.x2, f.y2, f.width, f.peakElev, x, y)

	case topoRidge:
		return topoLinearContrib(f.cx, f.cy, f.x2, f.y2, f.width, f.peakElev, x, y)

	case topoShelf:
		if x <= f.cx {
			return 0
		}
		if x >= f.x2 {
			return (f.x2 - f.cx) * f.slope
		}
		return (x - f.cx) * f.slope

	default:
		return 0
	}
}

// topoLinearContrib computes elevation contribution for a linear feature (trench/ridge).
// Uses distance from point to line segment, with cosine cross-section.
func topoLinearContrib(x1, y1, x2, y2, width, peakElev, px, py float64) float64 {
	lx := x2 - x1
	ly := y2 - y1
	lenSq := lx*lx + ly*ly
	if lenSq < 0.001 {
		return 0
	}

	// Project point onto line segment, clamped to [0, 1]
	t := ((px-x1)*lx + (py-y1)*ly) / lenSq
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	// Closest point on segment
	closestX := x1 + t*lx
	closestY := y1 + t*ly

	// Perpendicular distance
	dx := px - closestX
	dy := py - closestY
	dist := math.Sqrt(dx*dx + dy*dy)

	halfWidth := width / 2.0
	if dist >= halfWidth {
		return 0
	}

	// Cosine cross-section
	u := dist / halfWidth
	return peakElev * 0.5 * (1.0 + math.Cos(u*math.Pi))
}

// topoNoise returns deterministic 2D value noise in range [-1, 1].
// Uses a hash-based approach seeded by the zone seed.
// Deterministic: same seed + same coordinates = same result, always.
func topoNoise(seed int64, x, y float64) float64 {
	ix := int64(math.Floor(x))
	iy := int64(math.Floor(y))

	fx := x - float64(ix)
	fy := y - float64(iy)

	// Smoothstep interpolation weights
	sx := fx * fx * (3.0 - 2.0*fx)
	sy := fy * fy * (3.0 - 2.0*fy)

	// Hash corners of the cell
	n00 := topoHash(seed, ix, iy)
	n10 := topoHash(seed, ix+1, iy)
	n01 := topoHash(seed, ix, iy+1)
	n11 := topoHash(seed, ix+1, iy+1)

	// Bilinear interpolation with smoothstep
	nx0 := n00 + sx*(n10-n00)
	nx1 := n01 + sx*(n11-n01)
	return nx0 + sy*(nx1-nx0)
}

// topoHash returns a deterministic pseudo-random value in [-1, 1] for grid cell (ix, iy).
func topoHash(seed, ix, iy int64) float64 {
	h := seed*374761393 + ix*668265263 + iy*2147483647
	h = (h ^ (h >> 13)) * 1274126177
	h = h ^ (h >> 16)
	return float64(h%1000000)/500000.0 - 1.0
}

// fnTopology — compute terrain elevation at a point in a zone.
// topology(zone_dbref, x, y) → elevation (float)
// Positive = above sea level (land), negative = below (water).
// Zero = sea level / shoreline.
func fnTopology(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 {
		buf.WriteString("0")
		return
	}

	zoneStr := strings.TrimSpace(args[0])
	if !strings.HasPrefix(zoneStr, "#") {
		buf.WriteString("#-1 INVALID ZONE")
		return
	}
	zoneNum := toInt(zoneStr[1:])
	zone := gamedb.DBRef(zoneNum)

	x := toFloat(args[1])
	y := toFloat(args[2])

	tz := topoParse(ctx, zone)
	elev := topoElevation(tz, x, y)

	writeFloat(buf, elev)
}

// fnTopoflush — clear cached topology for a zone (call after changing TOPO_* attrs).
// topoflush(zone_dbref) → 1
func fnTopoflush(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("0")
		return
	}
	zoneStr := strings.TrimSpace(args[0])
	if !strings.HasPrefix(zoneStr, "#") {
		buf.WriteString("0")
		return
	}
	zoneNum := toInt(zoneStr[1:])
	topoInvalidate(gamedb.DBRef(zoneNum))
	buf.WriteString("1")
}

// fnDepth — convenience wrapper: returns water depth (positive) or 0 if on land.
// depth(zone_dbref, x, y[, tide_offset]) → depth in meters (positive = underwater)
func fnDepth(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 {
		buf.WriteString("0")
		return
	}

	zoneStr := strings.TrimSpace(args[0])
	if !strings.HasPrefix(zoneStr, "#") {
		buf.WriteString("0")
		return
	}
	zoneNum := toInt(zoneStr[1:])
	zone := gamedb.DBRef(zoneNum)

	x := toFloat(args[1])
	y := toFloat(args[2])
	tideOffset := 0.0
	if len(args) > 3 {
		tideOffset = toFloat(args[3])
	}

	tz := topoParse(ctx, zone)
	elev := topoElevation(tz, x, y) + tideOffset

	if elev >= 0 {
		buf.WriteString("0")
	} else {
		writeFloat(buf, -elev)
	}
}

// fnIslandchk — check if a position is on land.
// islandchk(zone_dbref, x, y[, tide_offset]) → 1 (land) or 0 (water)
func fnIslandchk(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 {
		buf.WriteString("0")
		return
	}

	zoneStr := strings.TrimSpace(args[0])
	if !strings.HasPrefix(zoneStr, "#") {
		buf.WriteString("0")
		return
	}
	zoneNum := toInt(zoneStr[1:])
	zone := gamedb.DBRef(zoneNum)

	x := toFloat(args[1])
	y := toFloat(args[2])
	tideOffset := 0.0
	if len(args) > 3 {
		tideOffset = toFloat(args[3])
	}

	tz := topoParse(ctx, zone)
	elev := topoElevation(tz, x, y) + tideOffset

	if elev > 0 {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// topoMapTier maps an elevation to a display character and ANSI color.
// Returns (character, foreground ANSI escape).
func topoMapTier(elev float64) (byte, string) {
	if elev > 10 {
		return '^', "\033[1;32m" // bright green — high land
	}
	if elev > 0 {
		return '.', "\033[33m" // yellow — beach/shore
	}
	depth := -elev
	switch {
	case depth < 5:
		return '~', "\033[1;36m" // bright cyan — shallows
	case depth < 15:
		return '~', "\033[36m" // cyan — moderate
	case depth < 30:
		return '-', "\033[1;34m" // bright blue — deep
	case depth < 60:
		return '-', "\033[34m" // blue — very deep
	case depth < 100:
		return '=', "\033[35m" // magenta — abyss
	default:
		return '#', "\033[1;35m" // bright magenta — trench
	}
}

// fnTopomap — render a topology map as ANSI-colored ASCII.
// topomap(zone_dbref, x1, y1, x2, y2[, step])
// step = grid units per character (default 1)
// Returns multi-line string with ANSI colors.
func fnTopomap(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 5 {
		buf.WriteString("#-1 USAGE: topomap(zone, x1, y1, x2, y2[, step])")
		return
	}

	zoneStr := strings.TrimSpace(args[0])
	if !strings.HasPrefix(zoneStr, "#") {
		buf.WriteString("#-1 INVALID ZONE")
		return
	}
	zoneNum := toInt(zoneStr[1:])
	zone := gamedb.DBRef(zoneNum)

	x1 := toFloat(args[1])
	y1 := toFloat(args[2])
	x2 := toFloat(args[3])
	y2 := toFloat(args[4])
	step := 1.0
	if len(args) > 5 {
		step = toFloat(args[5])
		if step < 0.1 {
			step = 0.1
		}
	}

	// Clamp output size to prevent abuse
	cols := int((x2 - x1) / step)
	rows := int((y2 - y1) / step)
	if cols < 1 || rows < 1 {
		buf.WriteString("#-1 INVALID RANGE")
		return
	}
	if cols > 120 {
		cols = 120
	}
	if rows > 60 {
		rows = 60
	}

	tz := topoParse(ctx, zone)

	// Y-axis label width (for left margin)
	maxY := int(y2)
	labelW := len(fmt.Sprintf("%d", maxY))

	// Render top-down (high Y first, like a map)
	reset := "\033[0m"
	for row := rows - 1; row >= 0; row-- {
		gy := y1 + float64(row)*step

		// Y-axis label
		label := fmt.Sprintf("%*d", labelW, int(gy))
		buf.WriteString(label)
		buf.WriteByte(' ')

		prevColor := ""
		for col := 0; col < cols; col++ {
			gx := x1 + float64(col)*step
			elev := topoElevation(tz, gx, gy)
			ch, color := topoMapTier(elev)
			if color != prevColor {
				buf.WriteString(color)
				prevColor = color
			}
			buf.WriteByte(ch)
		}
		buf.WriteString(reset)
		if row > 0 {
			buf.WriteByte('\n')
		}
	}

	// X-axis labels on last line
	buf.WriteByte('\n')
	// Spacer for Y-label column
	for i := 0; i < labelW+1; i++ {
		buf.WriteByte(' ')
	}
	// Print X labels at intervals
	xInterval := 10
	if step > 5 {
		xInterval = int(step) * 2
	}
	lastLabelEnd := 0
	for col := 0; col < cols; col++ {
		gx := int(x1 + float64(col)*step)
		if gx%xInterval == 0 && col >= lastLabelEnd {
			label := fmt.Sprintf("%d", gx)
			buf.WriteString(label)
			lastLabelEnd = col + len(label) + 1
		} else if col >= lastLabelEnd {
			buf.WriteByte(' ')
		}
	}
}

// topoGradient computes the terrain gradient (∂elev/∂x, ∂elev/∂y) at a point.
// Uses central difference with step h. Gradient points uphill.
func topoGradient(tz *topoZone, x, y, h float64) (float64, float64) {
	dEdx := (topoElevation(tz, x+h, y) - topoElevation(tz, x-h, y)) / (2 * h)
	dEdy := (topoElevation(tz, x, y+h) - topoElevation(tz, x, y-h)) / (2 * h)
	return dEdx, dEdy
}

// currentVector computes the flow vector at a position.
// Returns (dx, dy) per tick. Combines base flow, topology deflection, and tidal component.
// tidePhase is -1.0 to 1.0 (from external tide calculation).
func currentVector(tz *topoZone, x, y, tidePhase float64) (float64, float64) {
	// Start with prevailing current
	cx, cy := tz.currBaseX, tz.currBaseY

	// Topology deflection: flow bends around terrain features
	if tz.currDeflect > 0 && (cx != 0 || cy != 0) {
		gx, gy := topoGradient(tz, x, y, 1.0)
		gradMag := math.Sqrt(gx*gx + gy*gy)

		if gradMag > 0.001 {
			// Normalize gradient
			gnx := gx / gradMag
			gny := gy / gradMag

			// Perpendicular to gradient (flow goes around, not through)
			// Choose the perpendicular direction that aligns with base flow
			perpX, perpY := -gny, gnx
			dot := perpX*cx + perpY*cy
			if dot < 0 {
				perpX, perpY = gny, -gnx
			}

			// Deflection strength scales with gradient magnitude
			// Clamp gradMag contribution to avoid extreme deflection
			deflectStr := math.Min(gradMag*tz.currDeflect, 1.0)

			// Blend base flow toward perpendicular direction
			cx = cx*(1-deflectStr) + perpX*math.Sqrt(cx*cx+cy*cy)*deflectStr
			cy = cy*(1-deflectStr) + perpY*math.Sqrt(cx*cx+cy*cy)*deflectStr
		}

		// Speed reduction near land — friction from shallow water / terrain
		elev := topoElevation(tz, x, y)
		if elev > -5 && elev <= 0 {
			// Shallow water: reduce current proportionally
			factor := -elev / 5.0 // 0 at shore, 1 at depth 5
			cx *= factor
			cy *= factor
		} else if elev > 0 {
			// On land: no current
			cx, cy = 0, 0
		}
	}

	// Tidal component: oscillates with tide phase
	if tz.currTidal > 0 {
		tidalStr := tz.currTidal * tidePhase
		cx += math.Sin(tz.currTidalHd) * tidalStr
		cy += math.Cos(tz.currTidalHd) * tidalStr
	}

	return cx, cy
}

// fnCurrent — get flow vector at a position.
// current(zone_dbref, x, y[, tide_phase]) → "dx dy"
// tide_phase: -1.0 (ebb) to 1.0 (flood), default 0.
func fnCurrent(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 {
		buf.WriteString("0 0")
		return
	}

	zoneStr := strings.TrimSpace(args[0])
	if !strings.HasPrefix(zoneStr, "#") {
		buf.WriteString("0 0")
		return
	}
	zoneNum := toInt(zoneStr[1:])
	zone := gamedb.DBRef(zoneNum)

	x := toFloat(args[1])
	y := toFloat(args[2])
	tidePhase := 0.0
	if len(args) > 3 {
		tidePhase = toFloat(args[3])
		if tidePhase < -1 {
			tidePhase = -1
		}
		if tidePhase > 1 {
			tidePhase = 1
		}
	}

	tz := topoParse(ctx, zone)
	dx, dy := currentVector(tz, x, y, tidePhase)

	writeFloat(buf, dx)
	buf.WriteByte(' ')
	writeFloat(buf, dy)
}

