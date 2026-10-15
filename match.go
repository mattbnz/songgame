package main

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	parenRE = regexp.MustCompile(`\s*[\(\[][^)\]]*[\)\]]`)
	featRE  = regexp.MustCompile(`(?i)\s+(feat\.?|ft\.?|featuring|with)\s+.*$`)
	dashRE  = regexp.MustCompile(`\s+-\s+.*$`) // e.g. "Song - Remastered 2011"
)

// normalize strips parenthetical/bracketed content, trailing "- remaster" annotations,
// "feat." credits, accents, punctuation, and leading articles; lowercases & collapses whitespace.
func normalize(s string) string {
	s = strings.ToLower(s)
	s = parenRE.ReplaceAllString(s, "")
	s = dashRE.ReplaceAllString(s, "")
	s = featRE.ReplaceAllString(s, "")

	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case unicode.IsSpace(r), r == '-':
			b.WriteRune(' ')
		}
	}
	s = b.String()
	s = strings.Join(numberWordsToDigits(strings.Fields(s)), " ")
	s = strings.TrimPrefix(s, "the ")
	s = strings.TrimPrefix(s, "a ")
	s = strings.TrimPrefix(s, "an ")
	return s
}

var (
	unitWords = map[string]int{
		"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
		"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
		"thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16,
		"seventeen": 17, "eighteen": 18, "nineteen": 19,
	}
	tensWords = map[string]int{
		"twenty": 20, "thirty": 30, "forty": 40, "fifty": 50,
		"sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90,
	}
	ordinalUnitWords = map[string]int{
		"zeroth": 0, "first": 1, "second": 2, "third": 3, "fourth": 4, "fifth": 5,
		"sixth": 6, "seventh": 7, "eighth": 8, "ninth": 9, "tenth": 10,
		"eleventh": 11, "twelfth": 12, "thirteenth": 13, "fourteenth": 14,
		"fifteenth": 15, "sixteenth": 16, "seventeenth": 17, "eighteenth": 18,
		"nineteenth": 19,
	}
	ordinalTensWords = map[string]int{
		"twentieth": 20, "thirtieth": 30, "fortieth": 40, "fiftieth": 50,
		"sixtieth": 60, "seventieth": 70, "eightieth": 80, "ninetieth": 90,
	}
	// "21st", "2nd", "3rd", "4th" -> captures the digits
	ordinalDigitsRE = regexp.MustCompile(`^(\d+)(st|nd|rd|th)$`)
)

// numberWordsToDigits rewrites spelled-out numbers as digits so that e.g.
// "five seconds of summer" and "5 seconds of summer" normalize identically.
// Handles 0-99, including "twenty one" style compounds. Ordinals are treated
// the same as cardinals, so "twenty first", "21st" and "21" all become "21".
func numberWordsToDigits(words []string) []string {
	out := make([]string, 0, len(words))
	for i := 0; i < len(words); i++ {
		w := words[i]
		if n, ok := tensWords[w]; ok {
			if i+1 < len(words) {
				next := words[i+1]
				if u, ok := unitWords[next]; ok && u >= 1 && u <= 9 {
					n += u
					i++
				} else if u, ok := ordinalUnitWords[next]; ok && u >= 1 && u <= 9 {
					n += u
					i++
				}
			}
			out = append(out, strconv.Itoa(n))
		} else if n, ok := unitWords[w]; ok {
			out = append(out, strconv.Itoa(n))
		} else if n, ok := ordinalUnitWords[w]; ok {
			out = append(out, strconv.Itoa(n))
		} else if n, ok := ordinalTensWords[w]; ok {
			out = append(out, strconv.Itoa(n))
		} else if m := ordinalDigitsRE.FindStringSubmatch(w); m != nil {
			out = append(out, m[1])
		} else {
			out = append(out, w)
		}
	}
	return out
}

func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// fuzzyMatch returns true if guess is close enough to actual (after normalization).
// Allows exact match, substring containment either way, or small edit distance relative to length.
func fuzzyMatch(guess, actual string) bool {
	g := normalize(guess)
	a := normalize(actual)
	if g == "" || a == "" {
		return false
	}
	if g == a {
		return true
	}
	// containment only counts if the shorter side is reasonably long, to avoid "a" matching everything
	shortest := len(g)
	if len(a) < shortest {
		shortest = len(a)
	}
	if shortest >= 4 && (strings.Contains(g, a) || strings.Contains(a, g)) {
		return true
	}
	if closeEnough(g, a) {
		return true
	}
	// Typo-tolerant containment: compare the shorter side against each run of
	// the same number of words in the longer side, so e.g. "what a beautful name
	// it is" matches "what a beautiful name" and "hillsongs" matches "hillsong worship".
	gw, aw := strings.Fields(g), strings.Fields(a)
	short, long := gw, aw
	if len(short) > len(long) {
		short, long = long, short
	}
	shortStr := strings.Join(short, " ")
	if len(shortStr) < 4 {
		return false
	}
	for i := 0; i+len(short) <= len(long); i++ {
		if closeEnough(shortStr, strings.Join(long[i:i+len(short)], " ")) {
			return true
		}
	}
	return false
}

// closeEnough reports whether a and b are within an edit distance that scales with length.
func closeEnough(a, b string) bool {
	shortest := len(a)
	if len(b) < shortest {
		shortest = len(b)
	}
	tol := 1
	if shortest >= 8 {
		tol = 2
	}
	if shortest >= 14 {
		tol = 3
	}
	return levenshtein(a, b) <= tol
}

// genericArtistWords are words that show up across many artist names and so
// don't identify an artist on their own, e.g. "worship" in "Hillsong Worship".
var genericArtistWords = map[string]bool{
	"worship": true, "band": true, "orchestra": true, "choir": true,
	"music": true, "group": true, "trio": true, "quartet": true,
	"project": true, "collective": true, "ensemble": true, "crew": true,
	"boys": true, "girls": true, "brothers": true, "sisters": true,
	"family": true, "singers": true, "kids": true, "live": true,
	"united": true, "church": true, "and": true, "of": true, "dj": true,
	"mc": true, "lil": true, "young": true, "big": true, "little": true,
	"symphony": true, "philharmonic": true, "quintet": true, "experience": true,
}

// onlyGenericWords reports whether every word of the normalized string is generic.
func onlyGenericWords(s string) bool {
	words := strings.Fields(s)
	if len(words) == 0 {
		return false
	}
	for _, w := range words {
		if !genericArtistWords[w] {
			return false
		}
	}
	return true
}

// matchAnyArtist returns true if guess matches any single artist of a multi-artist track.
// A guess made up only of generic words (e.g. "worship") must match a whole artist
// name rather than just part of one.
func matchAnyArtist(guess string, artists []string) bool {
	if g := normalize(guess); onlyGenericWords(g) {
		for _, a := range artists {
			if closeEnough(g, normalize(a)) {
				return true
			}
		}
		return false
	}
	for _, a := range artists {
		if fuzzyMatch(guess, a) {
			return true
		}
	}
	// Also try matching the joined form, e.g. "simon and garfunkel"
	joined := strings.Join(artists, " ")
	return fuzzyMatch(guess, joined)
}
