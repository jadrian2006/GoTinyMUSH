package functions

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Flight/navigation functions for 3D grid-based movement system.
//
// Heading system: 32 compass points, 0=East, counterclockwise.
//   0=E, 4=NE, 8=N, 12=NW, 16=W, 20=SW, 24=S, 28=SE
//   Each step = 11.25 degrees.
//
// Grid system: 4 quadrants (NE, NW, SE, SW)
//   Letters AA-ZZ (W→E within each quadrant), 676 positions per quadrant
//   Numbers 000-999 (S→N within each quadrant), 1000 positions per quadrant
//   Altitude 0-100
//   Address format: LL-NNN-QQ (e.g. EL-453-NE)
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

// absToGrid converts absolute x, y to grid location string "LL NNN QQ".
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
	s = strings.TrimSpace(s)
	// Try dash-delimited first: EL-453-NE
	parts := strings.Split(s, "-")
	if len(parts) == 3 {
		num := toInt(parts[1])
		return gridToAbs(parts[0], num, parts[2])
	}
	// Try space-delimited: EL 453 NE
	parts = strings.Fields(s)
	if len(parts) == 3 {
		num := toInt(parts[1])
		return gridToAbs(parts[0], num, parts[2])
	}
	return 0, 0, false
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

// --- Multi-object tactical flight functions ---

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
