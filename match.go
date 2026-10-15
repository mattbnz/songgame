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
		case unicode.IsSpace(r):
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
)

// numberWordsToDigits rewrites spelled-out numbers as digits so that e.g.
// "five seconds of summer" and "5 seconds of summer" normalize identically.
// Handles 0-99, including "twenty one" style compounds.
func numberWordsToDigits(words []string) []string {
	out := make([]string, 0, len(words))
	for i := 0; i < len(words); i++ {
		w := words[i]
		if n, ok := tensWords[w]; ok {
			if i+1 < len(words) {
				if u, ok := unitWords[words[i+1]]; ok && u >= 1 && u <= 9 {
					n += u
					i++
				}
			}
			out = append(out, strconv.Itoa(n))
		} else if n, ok := unitWords[w]; ok {
			out = append(out, strconv.Itoa(n))
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
	// edit distance tolerance scales with length
	tol := 1
	if shortest >= 8 {
		tol = 2
	}
	if shortest >= 14 {
		tol = 3
	}
	return levenshtein(g, a) <= tol
}

// matchAnyArtist returns true if guess matches any single artist of a multi-artist track.
func matchAnyArtist(guess string, artists []string) bool {
	for _, a := range artists {
		if fuzzyMatch(guess, a) {
			return true
		}
	}
	// Also try matching the joined form, e.g. "simon and garfunkel"
	joined := strings.Join(artists, " ")
	return fuzzyMatch(guess, joined)
}
