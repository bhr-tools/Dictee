package main

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const nbsp = " "

// Spoken command -> symbol. Longest phrases are tried first.
var commands = map[string]string{
	"point d'interrogation": "?",
	"point d'exclamation":   "!",
	"points de suspension":  "…",
	"point-virgule":         ";",
	"point virgule":         ";",
	"deux points":           ":",
	"nouveau paragraphe":    "\n\n",
	"à la ligne":            "\n",
	"a la ligne":            "\n",
	"nouvelle ligne":        "\n",
	"ouvrez la parenthèse":  "(",
	"ouvrez la parenthese":  "(",
	"ouvre la parenthèse":   "(",
	"fermez la parenthèse":  ")",
	"fermez la parenthese":  ")",
	"ferme la parenthèse":   ")",
	"ouvrez les guillemets": "«",
	"ouvrez le guillemet":   "«",
	"fermez les guillemets": "»",
	"fermez le guillemet":   "»",
	"point":                 ".",
	"virgule":               ",",
	"puce":                  "–",
}

var maxPhrase = func() int {
	m := 0
	for k := range commands {
		if n := len(strings.Fields(k)); n > m {
			m = n
		}
	}
	return m
}()

// Words whose trailing period is part of the word, not a sentence end.
var abbreviations = map[string]bool{
	"m": true, "mm": true, "mme": true, "mmes": true, "mlle": true, "me": true,
	"art": true, "al": true, "cf": true, "c": true, "civ": true, "pén": true,
	"pen": true, "proc": true, "com": true, "n°": true, "nº": true, "p": true,
	"pp": true, "éd": true, "ed": true, "dr": true, "etc": true, "s": true,
}

var noise = regexp.MustCompile(`\[[^\]]*\]|\*[^*]*\*|\([^)]*\)`)

type item struct {
	text string
	sym  bool
}

// Format turns a raw whisper transcript, in which punctuation was dictated
// aloud, into typeset French text.
func Format(raw string) string {
	raw = strings.ReplaceAll(noise.ReplaceAllString(raw, " "), "’", "'")
	out := render(numbers(parse(strings.Fields(raw))))
	out = reference.ReplaceAllString(out, "$1$2."+nbsp+"$3")
	out = euros.ReplaceAllString(out, "$1"+nbsp+"$2"+nbsp+"€")
	return eurosShort.ReplaceAllString(out, "$1"+nbsp+"€")
}

// euros groups thousands and spaces the sign in amounts whisper wrote as digits.
var euros = regexp.MustCompile(`(\d)(\d{3})[\s\x{a0}]?€`)
var eurosShort = regexp.MustCompile(`(\d)\s€`)

var reference = regexp.MustCompile(`(?i)(articles?\s)([LRD])\.?\s?(\d)`)

// parse maps tokens to words and symbols. Whisper's own edge punctuation is
// kept (it often turns a spoken "virgule" into "," by itself), but a dictated
// punctuation command replaces whisper's marks right before it.
func parse(tokens []string) []item {
	type tok struct{ word, tail string }
	toks := make([]tok, len(tokens))
	cores := make([]string, len(tokens))
	for i, t := range tokens {
		toks[i].word, toks[i].tail = splitEdges(t)
		cores[i] = strings.ToLower(toks[i].word)
	}
	var out []item
	whisper := 0 // trailing items in out that came from whisper's own marks
	for i := 0; i < len(toks); {
		matched := false
		for n := min(maxPhrase, len(toks)-i); n >= 1; n-- {
			s, ok := commands[strings.Join(cores[i:i+n], " ")]
			if !ok {
				continue
			}
			if s != "\n" && s != "\n\n" {
				out = out[:len(out)-whisper]
			}
			out = append(out, item{s, true})
			whisper = 0
			i += n
			matched = true
			break
		}
		if matched {
			continue
		}
		whisper = 0
		if toks[i].word != "" {
			out = append(out, item{toks[i].word, false})
		}
		for _, m := range marks(toks[i].tail) {
			out = append(out, item{m, true})
			whisper++
		}
		i++
	}
	// the same mark twice in a row (whisper's plus the dictated one) is one mark
	dedup := out[:0]
	for _, it := range out {
		if n := len(dedup); n > 0 && it.sym && dedup[n-1] == it && !strings.HasPrefix(it.text, "\n") {
			continue
		}
		dedup = append(dedup, it)
	}
	return dedup
}

// splitEdges separates a token into its word and trailing punctuation.
// Abbreviations such as "M." keep their period.
func splitEdges(t string) (word, tail string) {
	const edge = ",.;:!?…"
	word = strings.TrimLeft(t, edge)
	core := strings.TrimRight(word, edge)
	tail = word[len(core):]
	if strings.HasPrefix(tail, ".") && abbreviations[strings.ToLower(core)] {
		return core + ".", tail[1:]
	}
	return core, tail
}

func marks(tail string) []string {
	tail = strings.ReplaceAll(tail, "...", "…")
	var m []string
	for _, r := range tail {
		m = append(m, string(r))
	}
	return m
}

// render applies French typography: spacing around marks and capitals.
func render(items []item) string {
	var b strings.Builder
	space, capNext := false, true
	for _, it := range items {
		if !it.sym {
			if space {
				b.WriteByte(' ')
			}
			w := it.text
			if capNext {
				w = upperFirst(w)
			}
			b.WriteString(w)
			space, capNext = true, false
			continue
		}
		switch it.text {
		case ",", ".", "…", ")":
			b.WriteString(it.text)
			space = true
			capNext = it.text == "."
		case ";", ":", "?", "!":
			b.WriteString(nbsp + it.text)
			space = true
			capNext = it.text == "?" || it.text == "!"
		case "(", "«", "–":
			if space {
				b.WriteByte(' ')
			}
			b.WriteString(it.text)
			if it.text == "«" {
				b.WriteString(nbsp)
				space = false
			} else {
				space = it.text == "–"
			}
		case "»":
			b.WriteString(nbsp + "»")
			space = true
		default: // newlines
			b.WriteString(it.text)
			space, capNext = false, true
		}
	}
	return strings.TrimSpace(b.String())
}

func upperFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// --- Numbers -------------------------------------------------------------

var numWords = map[string]int{
	"zéro": 0, "un": 1, "une": 1, "deux": 2, "trois": 3, "quatre": 4, "cinq": 5,
	"six": 6, "sept": 7, "huit": 8, "neuf": 9, "dix": 10, "onze": 11, "douze": 12,
	"treize": 13, "quatorze": 14, "quinze": 15, "seize": 16, "vingt": 20,
	"vingts": 20, "trente": 30, "quarante": 40, "cinquante": 50, "soixante": 60,
	"cent": 100, "cents": 100, "mille": 1000,
}

var months = map[string]bool{
	"janvier": true, "février": true, "mars": true, "avril": true, "mai": true,
	"juin": true, "juillet": true, "août": true, "septembre": true,
	"octobre": true, "novembre": true, "décembre": true,
}

// parts splits a token like "vingt-six" into number words; ok is false if any
// part is not a number word.
func parts(tok string) (p []string, ok bool) {
	for _, s := range strings.Split(strings.ToLower(tok), "-") {
		if _, is := numWords[s]; !is && s != "et" {
			return nil, false
		}
		p = append(p, s)
	}
	return p, true
}

// numbers rewrites runs of spelled-out numbers as digits. Small values are
// kept as words (legal style) unless followed by euros, a percent or a month.
func numbers(items []item) []item {
	var out []item
	for i := 0; i < len(items); {
		if items[i].sym {
			out = append(out, items[i])
			i++
			continue
		}
		if items[i].text == "premier" && i+1 < len(items) && months[strings.ToLower(items[i+1].text)] {
			out = append(out, item{"1er", false})
			i++
			continue
		}
		var run []string
		j := i
		for ; j < len(items) && !items[j].sym; j++ {
			p, ok := parts(items[j].text)
			if !ok || (len(run) == 0 && p[0] == "et") {
				break
			}
			run = append(run, p...)
		}
		for j > i && len(run) > 0 && run[len(run)-1] == "et" && strings.EqualFold(items[j-1].text, "et") {
			run, j = run[:len(run)-1], j-1 // a trailing "et" is a conjunction
		}
		v, ok := parseNumber(run)
		if !ok {
			out = append(out, items[i])
			i++
			continue
		}
		next := ""
		if j < len(items) && !items[j].sym {
			next = strings.ToLower(items[j].text)
		}
		eur := next == "euros" || next == "euro"
		pct := next == "pour" && j+1 < len(items) && !items[j+1].sym && strings.EqualFold(items[j+1].text, "cent")
		if v < 10 && !eur && !pct && !months[next] {
			out = append(out, items[i:j]...)
			i = j
			continue
		}
		s := formatInt(v, eur || pct)
		switch {
		case eur:
			s += nbsp + "€"
			j++
		case pct:
			s += nbsp + "%"
			j += 2
		}
		out = append(out, item{s, false})
		i = j
	}
	return out
}

func parseNumber(p []string) (int, bool) {
	if len(p) == 0 {
		return 0, false
	}
	total, cur, last := 0, 0, 0
	for _, w := range p {
		if w == "et" {
			continue
		}
		v := numWords[w]
		switch {
		case w == "mille":
			if cur == 0 {
				cur = 1
			}
			total, cur = total+cur*1000, 0
		case v == 100:
			if cur == 0 {
				cur = 100
			} else if cur < 10 {
				cur *= 100
			} else {
				return 0, false
			}
		case v == 20 && last == 4 && cur%10 == 4:
			cur += 76 // quatre-vingt
		case v >= 1 && v <= 19 && last >= 1 && last <= 19 && last != 10 && !(v < 10 && last == 10):
			return 0, false // "trois quatre", "onze deux"
		default:
			cur += v
		}
		last = v
	}
	return total + cur, true
}

func formatInt(v int, unit bool) string {
	s := strconv.Itoa(v)
	if v < 1000 || (!unit && v >= 1900 && v <= 2100) {
		return s
	}
	var b []byte
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b = append(b, nbsp...)
		}
		b = append(b, s[i])
	}
	return string(b)
}
