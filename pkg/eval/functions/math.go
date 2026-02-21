package functions

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

func toFloat(s string) float64 {
	s = strings.TrimSpace(s)
	// Match C atof() behavior: parse leading numeric characters (including
	// decimal point), ignore trailing non-numeric text.
	end := 0
	if end < len(s) && (s[end] == '-' || s[end] == '+') {
		end++
	}
	sawDot := false
	for end < len(s) {
		if s[end] == '.' && !sawDot {
			sawDot = true
			end++
		} else if s[end] >= '0' && s[end] <= '9' {
			end++
		} else {
			break
		}
	}
	f, _ := strconv.ParseFloat(s[:end], 64)
	return f
}

func toInt(s string) int {
	s = strings.TrimSpace(s)
	// Match C atoi() behavior: parse leading digits, ignore trailing non-digits.
	// This is critical for MUSHcode like div(get(obj/attr), N) where the attr
	// value may contain trailing text (e.g. "9832 Raimier").
	neg := false
	i := 0
	if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
		if s[0] == '-' {
			neg = true
		}
		s = s[1:]
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		i = i*10 + int(c-'0')
	}
	if neg {
		return -i
	}
	return i
}

func boolToStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func writeInt(buf *strings.Builder, i int) {
	buf.WriteString(strconv.Itoa(i))
}

func writeFloat(buf *strings.Builder, f float64) {
	if f == float64(int64(f)) {
		buf.WriteString(strconv.FormatInt(int64(f), 10))
	} else {
		buf.WriteString(strconv.FormatFloat(f, 'f', 6, 64))
	}
}

// --- Arithmetic ---

// add() returns integer result (C TinyMUSH ival behavior: parse as float, compute, truncate).
func fnAdd(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	sum := 0.0
	for _, a := range args {
		sum += toFloat(a)
	}
	writeInt(buf, int(sum))
}

// sub() returns integer result (C TinyMUSH ival behavior: parse as float, compute, truncate).
func fnSub(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	writeInt(buf, int(toFloat(args[0])-toFloat(args[1])))
}

// mul() returns integer result (C TinyMUSH ival behavior: parse as float, compute, truncate).
func fnMul(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) == 0 { buf.WriteString("0"); return }
	prod := 1.0
	for _, a := range args {
		prod *= toFloat(a)
	}
	writeInt(buf, int(prod))
}

// fadd() returns float result.
func fnFadd(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	sum := 0.0
	for _, a := range args {
		sum += toFloat(a)
	}
	writeFloat(buf, sum)
}

// fsub() returns float result.
func fnFsub(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	writeFloat(buf, toFloat(args[0])-toFloat(args[1]))
}

// fmul() returns float result.
func fnFmul(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) == 0 { buf.WriteString("0"); return }
	prod := 1.0
	for _, a := range args {
		prod *= toFloat(a)
	}
	writeFloat(buf, prod)
}

func fnDiv(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	b := toInt(args[1])
	if b == 0 {
		buf.WriteString("#-1 DIVIDE BY ZERO")
		return
	}
	writeInt(buf, toInt(args[0])/b)
}

func fnFdiv(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	b := toFloat(args[1])
	if b == 0 {
		buf.WriteString("#-1 DIVIDE BY ZERO")
		return
	}
	writeFloat(buf, toFloat(args[0])/b)
}

func fnModulo(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	b := toInt(args[1])
	if b == 0 {
		buf.WriteString("#-1 DIVIDE BY ZERO")
		return
	}
	writeInt(buf, toInt(args[0])%b)
}

func fnAbs(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	f := toFloat(args[0])
	writeFloat(buf, math.Abs(f))
}

func fnSign(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	f := toFloat(args[0])
	if f > 0 { buf.WriteString("1") } else if f < 0 { buf.WriteString("-1") } else { buf.WriteString("0") }
}

func fnInc(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("1"); return }
	writeInt(buf, toInt(args[0])+1)
}

func fnDec(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("-1"); return }
	writeInt(buf, toInt(args[0])-1)
}

func fnRound(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	f := toFloat(args[0])
	places := toInt(args[1])
	mult := math.Pow(10, float64(places))
	result := math.Round(f*mult) / mult
	if places <= 0 {
		writeInt(buf, int(result))
	} else {
		buf.WriteString(strconv.FormatFloat(result, 'f', places, 64))
	}
}

func fnTrunc(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeInt(buf, int(toFloat(args[0])))
}

func fnFloor(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeInt(buf, int(math.Floor(toFloat(args[0]))))
}

func fnCeil(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeInt(buf, int(math.Ceil(toFloat(args[0]))))
}

func fnSqrt(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	f := toFloat(args[0])
	if f < 0 {
		buf.WriteString("#-1 SQUARE ROOT OF NEGATIVE")
		return
	}
	writeFloat(buf, math.Sqrt(f))
}

func fnPower(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	writeFloat(buf, math.Pow(toFloat(args[0]), toFloat(args[1])))
}

func fnMax(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) == 0 { buf.WriteString("0"); return }
	m := toFloat(args[0])
	for _, a := range args[1:] {
		v := toFloat(a)
		if v > m { m = v }
	}
	writeFloat(buf, m)
}

func fnMin(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) == 0 { buf.WriteString("0"); return }
	m := toFloat(args[0])
	for _, a := range args[1:] {
		v := toFloat(a)
		if v < m { m = v }
	}
	writeFloat(buf, m)
}

func fnPi(_ *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(fmt.Sprintf("%.6f", math.Pi))
}

func fnE(_ *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(fmt.Sprintf("%.6f", math.E))
}

// --- Comparison ---

func fnGt(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(boolToStr(toFloat(args[0]) > toFloat(args[1])))
}

func fnGte(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(boolToStr(toFloat(args[0]) >= toFloat(args[1])))
}

func fnLt(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(boolToStr(toFloat(args[0]) < toFloat(args[1])))
}

func fnLte(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(boolToStr(toFloat(args[0]) <= toFloat(args[1])))
}

func fnEq(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(boolToStr(toFloat(args[0]) == toFloat(args[1])))
}

func fnNeq(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(boolToStr(toFloat(args[0]) != toFloat(args[1])))
}

func fnNcomp(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	a, b := toFloat(args[0]), toFloat(args[1])
	if a < b { buf.WriteString("-1") } else if a > b { buf.WriteString("1") } else { buf.WriteString("0") }
}

// --- Logic ---

func fnAnd(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	for _, a := range args {
		if toInt(a) == 0 {
			buf.WriteString("0")
			return
		}
	}
	buf.WriteString("1")
}

func fnOr(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	for _, a := range args {
		if toInt(a) != 0 {
			buf.WriteString("1")
			return
		}
	}
	buf.WriteString("0")
}

func fnXor(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	count := 0
	for _, a := range args {
		if toInt(a) != 0 { count++ }
	}
	buf.WriteString(boolToStr(count%2 == 1))
}

func fnNot(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("1"); return }
	buf.WriteString(boolToStr(toInt(args[0]) == 0))
}

func fnNotBool(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("1"); return }
	buf.WriteString(boolToStr(strings.TrimSpace(args[0]) == "" || args[0] == "0"))
}

func fnT(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	s := strings.TrimSpace(args[0])
	if s == "" || s == "0" {
		buf.WriteString("0")
	} else {
		buf.WriteString("1")
	}
}

// --- Trigonometric functions ---

func fnSin(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Sin(toFloat(args[0])))
}

func fnSind(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Sin(toFloat(args[0])*math.Pi/180))
}

func fnCos(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Cos(toFloat(args[0])))
}

func fnCosd(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Cos(toFloat(args[0])*math.Pi/180))
}

func fnTan(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Tan(toFloat(args[0])))
}

func fnTand(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Tan(toFloat(args[0])*math.Pi/180))
}

func fnAsin(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	v := toFloat(args[0])
	if v < -1 || v > 1 {
		buf.WriteString("#-1 ARCSINE ARGUMENT OUT OF RANGE")
		return
	}
	writeFloat(buf, math.Asin(v))
}

func fnAsind(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	v := toFloat(args[0])
	if v < -1 || v > 1 {
		buf.WriteString("#-1 ARCSINE ARGUMENT OUT OF RANGE")
		return
	}
	writeFloat(buf, math.Asin(v)*180/math.Pi)
}

func fnAcos(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	v := toFloat(args[0])
	if v < -1 || v > 1 {
		buf.WriteString("#-1 ARCCOSINE ARGUMENT OUT OF RANGE")
		return
	}
	writeFloat(buf, math.Acos(v))
}

func fnAcosd(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	v := toFloat(args[0])
	if v < -1 || v > 1 {
		buf.WriteString("#-1 ARCCOSINE ARGUMENT OUT OF RANGE")
		return
	}
	writeFloat(buf, math.Acos(v)*180/math.Pi)
}

func fnAtan(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Atan(toFloat(args[0])))
}

func fnAtand(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Atan(toFloat(args[0]))*180/math.Pi)
}

// --- Exponential/Logarithmic ---

func fnExp(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Exp(toFloat(args[0])))
}

func fnLn(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	v := toFloat(args[0])
	if v <= 0 {
		buf.WriteString("#-1 LOG OF NEGATIVE OR ZERO")
		return
	}
	writeFloat(buf, math.Log(v))
}

func fnLog(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	v := toFloat(args[0])
	if v <= 0 {
		buf.WriteString("#-1 LOG OF NEGATIVE OR ZERO")
		return
	}
	writeFloat(buf, math.Log10(v))
}

// --- Bitwise ---

func fnShl(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	writeInt(buf, toInt(args[0])<<uint(toInt(args[1])))
}

func fnShr(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	writeInt(buf, toInt(args[0])>>uint(toInt(args[1])))
}

func fnBand(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	writeInt(buf, toInt(args[0])&toInt(args[1]))
}

func fnBor(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	writeInt(buf, toInt(args[0])|toInt(args[1]))
}

func fnBnand(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	writeInt(buf, toInt(args[0]) & ^toInt(args[1]))
}

// --- Additional math ---

func fnFloordiv(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	b := toFloat(args[1])
	if b == 0 {
		buf.WriteString("#-1 DIVIDE BY ZERO")
		return
	}
	writeInt(buf, int(math.Floor(toFloat(args[0])/b)))
}

func fnDist2d(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 4 { buf.WriteString("0"); return }
	dx := toFloat(args[2]) - toFloat(args[0])
	dy := toFloat(args[3]) - toFloat(args[1])
	writeFloat(buf, math.Sqrt(dx*dx+dy*dy))
}

func fnDist3d(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 6 { buf.WriteString("0"); return }
	dx := toFloat(args[3]) - toFloat(args[0])
	dy := toFloat(args[4]) - toFloat(args[1])
	dz := toFloat(args[5]) - toFloat(args[2])
	writeFloat(buf, math.Sqrt(dx*dx+dy*dy+dz*dz))
}

// --- Alpha comparison ---

func fnAlphamax(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) == 0 { return }
	m := args[0]
	for _, a := range args[1:] {
		if strings.ToLower(a) > strings.ToLower(m) { m = a }
	}
	buf.WriteString(m)
}

func fnAlphamin(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) == 0 { return }
	m := args[0]
	for _, a := range args[1:] {
		if strings.ToLower(a) < strings.ToLower(m) { m = a }
	}
	buf.WriteString(m)
}

// --- List math ---

func fnLmax(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 1 { delim = args[1] }
	words := splitList(args[0], delim)
	if len(words) == 0 { buf.WriteString("0"); return }
	m := toFloat(words[0])
	for _, w := range words[1:] {
		v := toFloat(w)
		if v > m { m = v }
	}
	writeFloat(buf, m)
}

func fnLmin(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 1 { delim = args[1] }
	words := splitList(args[0], delim)
	if len(words) == 0 { buf.WriteString("0"); return }
	m := toFloat(words[0])
	for _, w := range words[1:] {
		v := toFloat(w)
		if v < m { m = v }
	}
	writeFloat(buf, m)
}

// --- Logic variants ---

// ANDBOOL — treats args as boolean (empty string = false, non-empty = true)
func fnAndbool(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	for _, a := range args {
		s := strings.TrimSpace(a)
		if s == "" || s == "0" {
			buf.WriteString("0")
			return
		}
	}
	buf.WriteString("1")
}

// ORBOOL — treats args as boolean
func fnOrbool(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	for _, a := range args {
		s := strings.TrimSpace(a)
		if s != "" && s != "0" {
			buf.WriteString("1")
			return
		}
	}
	buf.WriteString("0")
}

// XORBOOL — treats args as boolean
func fnXorbool(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	count := 0
	for _, a := range args {
		s := strings.TrimSpace(a)
		if s != "" && s != "0" { count++ }
	}
	buf.WriteString(boolToStr(count%2 == 1))
}

// CAND — short-circuit AND (noeval)
func fnCand(ctx *eval.EvalContext, args []string, buf *strings.Builder, player, cause gamedb.DBRef) {
	for _, a := range args {
		val := ctx.Exec(a, eval.EvFCheck|eval.EvEval, nil)
		if toInt(val) == 0 {
			buf.WriteString("0")
			return
		}
	}
	buf.WriteString("1")
}

// CANDBOOL — short-circuit AND with boolean semantics
func fnCandbool(ctx *eval.EvalContext, args []string, buf *strings.Builder, player, cause gamedb.DBRef) {
	for _, a := range args {
		val := ctx.Exec(a, eval.EvFCheck|eval.EvEval, nil)
		s := strings.TrimSpace(val)
		if s == "" || s == "0" {
			buf.WriteString("0")
			return
		}
	}
	buf.WriteString("1")
}

// COR — short-circuit OR (noeval)
func fnCor(ctx *eval.EvalContext, args []string, buf *strings.Builder, player, cause gamedb.DBRef) {
	for _, a := range args {
		val := ctx.Exec(a, eval.EvFCheck|eval.EvEval, nil)
		if toInt(val) != 0 {
			buf.WriteString("1")
			return
		}
	}
	buf.WriteString("0")
}

// CORBOOL — short-circuit OR with boolean semantics
func fnCorbool(ctx *eval.EvalContext, args []string, buf *strings.Builder, player, cause gamedb.DBRef) {
	for _, a := range args {
		val := ctx.Exec(a, eval.EvFCheck|eval.EvEval, nil)
		s := strings.TrimSpace(val)
		if s != "" && s != "0" {
			buf.WriteString("1")
			return
		}
	}
	buf.WriteString("0")
}

// LANDBOOL — list AND with boolean semantics
func fnLandbool(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("1"); return }
	delim := " "
	if len(args) > 1 { delim = args[1] }
	words := splitList(args[0], delim)
	for _, w := range words {
		s := strings.TrimSpace(w)
		if s == "" || s == "0" {
			buf.WriteString("0")
			return
		}
	}
	buf.WriteString("1")
}

// LORBOOL — list OR with boolean semantics
func fnLorbool(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 1 { delim = args[1] }
	words := splitList(args[0], delim)
	for _, w := range words {
		s := strings.TrimSpace(w)
		if s != "" && s != "0" {
			buf.WriteString("1")
			return
		}
	}
	buf.WriteString("0")
}

// --- Vector math ---

func parseVector(s string) []float64 {
	words := strings.Fields(strings.TrimSpace(s))
	vec := make([]float64, len(words))
	for i, w := range words {
		vec[i] = toFloat(w)
	}
	return vec
}

func writeVector(buf *strings.Builder, v []float64) {
	for i, f := range v {
		if i > 0 { buf.WriteByte(' ') }
		writeFloat(buf, f)
	}
}

func fnVadd(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	a := parseVector(args[0])
	b := parseVector(args[1])
	if len(a) != len(b) {
		buf.WriteString("#-1 VECTORS MUST BE SAME DIMENSIONS")
		return
	}
	r := make([]float64, len(a))
	for i := range a { r[i] = a[i] + b[i] }
	writeVector(buf, r)
}

func fnVsub(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	a := parseVector(args[0])
	b := parseVector(args[1])
	if len(a) != len(b) {
		buf.WriteString("#-1 VECTORS MUST BE SAME DIMENSIONS")
		return
	}
	r := make([]float64, len(a))
	for i := range a { r[i] = a[i] - b[i] }
	writeVector(buf, r)
}

func fnVmul(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	a := parseVector(args[0])
	scalar := toFloat(args[1])
	r := make([]float64, len(a))
	for i := range a { r[i] = a[i] * scalar }
	writeVector(buf, r)
}

func fnVdot(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	a := parseVector(args[0])
	b := parseVector(args[1])
	if len(a) != len(b) {
		buf.WriteString("#-1 VECTORS MUST BE SAME DIMENSIONS")
		return
	}
	sum := 0.0
	for i := range a { sum += a[i] * b[i] }
	writeFloat(buf, sum)
}

func fnVmag(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	a := parseVector(args[0])
	sum := 0.0
	for _, v := range a { sum += v * v }
	writeFloat(buf, math.Sqrt(sum))
}

func fnVunit(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	a := parseVector(args[0])
	sum := 0.0
	for _, v := range a { sum += v * v }
	mag := math.Sqrt(sum)
	if mag == 0 {
		buf.WriteString("#-1 ZERO-LENGTH VECTOR")
		return
	}
	r := make([]float64, len(a))
	for i := range a { r[i] = a[i] / mag }
	writeVector(buf, r)
}

func fnVdim(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	a := parseVector(args[0])
	writeInt(buf, len(a))
}

// fnVcross — 3D cross product.
func fnVcross(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	a := parseVector(args[0])
	b := parseVector(args[1])
	if len(a) != 3 || len(b) != 3 {
		buf.WriteString("#-1 VECTORS MUST BE 3D")
		return
	}
	r := []float64{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
	writeVector(buf, r)
}

// fnVdist — N-dimensional distance between two points.
func fnVdist(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	a := parseVector(args[0])
	b := parseVector(args[1])
	if len(a) != len(b) {
		buf.WriteString("#-1 VECTORS MUST BE SAME DIMENSIONS")
		return
	}
	sum := 0.0
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	writeFloat(buf, math.Sqrt(sum))
}

// fnVlerp — linear interpolation between two vectors.
// vlerp(v1, v2, t) — t=0 returns v1, t=1 returns v2.
func fnVlerp(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	a := parseVector(args[0])
	b := parseVector(args[1])
	t := toFloat(args[2])
	if len(a) != len(b) {
		buf.WriteString("#-1 VECTORS MUST BE SAME DIMENSIONS")
		return
	}
	r := make([]float64, len(a))
	for i := range a {
		r[i] = a[i] + t*(b[i]-a[i])
	}
	writeVector(buf, r)
}

// fnVnear — proximity test: returns 1 if v2 is within radius of v1.
func fnVnear(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("0"); return }
	a := parseVector(args[0])
	b := parseVector(args[1])
	radius := toFloat(args[2])
	if len(a) != len(b) {
		buf.WriteString("#-1 VECTORS MUST BE SAME DIMENSIONS")
		return
	}
	sum := 0.0
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	if math.Sqrt(sum) <= radius {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// fnVclamp — clamp each component of a vector to min/max bounds.
// vclamp(v, min, max) — each arg is a vector of the same dimension.
func fnVclamp(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	v := parseVector(args[0])
	lo := parseVector(args[1])
	hi := parseVector(args[2])
	if len(v) != len(lo) || len(v) != len(hi) {
		buf.WriteString("#-1 VECTORS MUST BE SAME DIMENSIONS")
		return
	}
	r := make([]float64, len(v))
	for i := range v {
		r[i] = math.Max(lo[i], math.Min(hi[i], v[i]))
	}
	writeVector(buf, r)
}

// fnAtan2 — two-argument arctangent (radians).
func fnAtan2(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	y := toFloat(args[0])
	x := toFloat(args[1])
	writeFloat(buf, math.Atan2(y, x))
}

// fnBound — clamp a scalar value to [min, max].
// bound(value, min, max)
func fnBound(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("0"); return }
	val := toFloat(args[0])
	lo := toFloat(args[1])
	hi := toFloat(args[2])
	writeFloat(buf, math.Max(lo, math.Min(hi, val)))
}

// fnAvg — average of a list of numbers.
// avg(n1, n2, ...)
func fnAvg(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) == 0 { buf.WriteString("0"); return }
	sum := 0.0
	for _, a := range args {
		sum += toFloat(a)
	}
	writeFloat(buf, sum/float64(len(args)))
}

// fnMedian — median of a list of numbers.
// median(n1, n2, ...)
func fnMedian(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) == 0 { buf.WriteString("0"); return }
	vals := make([]float64, len(args))
	for i, a := range args {
		vals[i] = toFloat(a)
	}
	// Sort
	for i := 0; i < len(vals); i++ {
		for j := i + 1; j < len(vals); j++ {
			if vals[j] < vals[i] {
				vals[i], vals[j] = vals[j], vals[i]
			}
		}
	}
	n := len(vals)
	if n%2 == 0 {
		writeFloat(buf, (vals[n/2-1]+vals[n/2])/2)
	} else {
		writeFloat(buf, vals[n/2])
	}
}

// --- RhostMUSH extension math functions ---

// fnBetween — test if a value is between two bounds (inclusive).
// between(lower, upper, value) → 1 if lower <= value <= upper, 0 otherwise
func fnBetween(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("0"); return }
	lo := toFloat(args[0])
	hi := toFloat(args[1])
	val := toFloat(args[2])
	if val >= lo && val <= hi {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// fnCosh — hyperbolic cosine.
func fnCosh(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Cosh(toFloat(args[0])))
}

// fnSinh — hyperbolic sine.
func fnSinh(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Sinh(toFloat(args[0])))
}

// fnTanh — hyperbolic tangent.
func fnTanh(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	writeFloat(buf, math.Tanh(toFloat(args[0])))
}

// fnFmod — floating-point modulo.
// fmod(x, y) → x mod y (floating point)
func fnFmod(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	x := toFloat(args[0])
	y := toFloat(args[1])
	if y == 0 { buf.WriteString("#-1 DIVIDE BY ZERO"); return }
	writeFloat(buf, math.Mod(x, y))
}

// fnTobin — convert integer to binary string.
// tobin(number) → binary representation
func fnTobin(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	n := toInt(args[0])
	if n < 0 {
		// Two's complement 32-bit
		buf.WriteString(strconv.FormatUint(uint64(uint32(n)), 2))
	} else {
		buf.WriteString(strconv.FormatInt(int64(n), 2))
	}
}

// fnTodec — convert to decimal (identity for base-10 input, or from other bases).
// todec(number) → decimal representation
func fnTodec(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	s := strings.TrimSpace(args[0])
	// Try parsing as hex (0x prefix), octal (0o prefix), or binary (0b prefix)
	var n int64
	var err error
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, err = strconv.ParseInt(s[2:], 16, 64)
	} else if strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O") {
		n, err = strconv.ParseInt(s[2:], 8, 64)
	} else if strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") {
		n, err = strconv.ParseInt(s[2:], 2, 64)
	} else {
		n, err = strconv.ParseInt(s, 10, 64)
	}
	if err != nil {
		buf.WriteString("#-1 INVALID NUMBER")
		return
	}
	buf.WriteString(strconv.FormatInt(n, 10))
}

// fnTohex — convert integer to hexadecimal.
// tohex(number) → hex representation (uppercase, no 0x prefix)
func fnTohex(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	n := toInt(args[0])
	if n < 0 {
		buf.WriteString(strings.ToUpper(strconv.FormatUint(uint64(uint32(n)), 16)))
	} else {
		buf.WriteString(strings.ToUpper(strconv.FormatInt(int64(n), 16)))
	}
}

// fnTooct — convert integer to octal.
// tooct(number) → octal representation
func fnTooct(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	n := toInt(args[0])
	if n < 0 {
		buf.WriteString(strconv.FormatUint(uint64(uint32(n)), 8))
	} else {
		buf.WriteString(strconv.FormatInt(int64(n), 8))
	}
}

// fnRoman — convert integer to Roman numerals.
// roman(number) → Roman numeral string (1-3999999)
func fnRoman(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	n := toInt(args[0])
	if n <= 0 || n > 3999999 {
		buf.WriteString("#-1 OUT OF RANGE")
		return
	}
	type pair struct { val int; sym string }
	numerals := []pair{
		{1000000, "M\u0305"}, {900000, "C\u0305M\u0305"}, {500000, "D\u0305"}, {400000, "C\u0305D\u0305"},
		{100000, "C\u0305"}, {90000, "X\u0305C\u0305"}, {50000, "L\u0305"}, {40000, "X\u0305L\u0305"},
		{10000, "X\u0305"}, {9000, "MX\u0305"}, {5000, "V\u0305"}, {4000, "MV\u0305"},
		{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"},
		{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
		{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
	}
	for _, p := range numerals {
		for n >= p.val {
			buf.WriteString(p.sym)
			n -= p.val
		}
	}
}

// fnNand — NAND logic gate.
// nand(val1, val2, ...) → !(val1 AND val2 AND ...)
func fnNand(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	result := true
	for _, a := range args {
		if toInt(a) == 0 { result = false; break }
	}
	if !result {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// fnNor — NOR logic gate.
// nor(val1, val2, ...) → !(val1 OR val2 OR ...)
func fnNor(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	anyTrue := false
	for _, a := range args {
		if toInt(a) != 0 { anyTrue = true; break }
	}
	if !anyTrue {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// fnXnor — XNOR logic gate (equality for booleans).
// xnor(val1, val2, ...) → 1 if all values have the same truth value
func fnXnor(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	count := 0
	for _, a := range args {
		if toInt(a) != 0 { count++ }
	}
	// XNOR: true if even number of true values (including 0)
	if count%2 == 0 {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}
