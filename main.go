package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// punctuation considered by the spec
func isP(r rune) bool {
	return r == '.' || r == ',' || r == '!' || r == '?' || r == ':' || r == ';'
}

// split leading/trailing punctuation from a token
func stripEdges(s string) (pre, core, suf string) {
	a, b := 0, len(s)
	for a < b && (isP(rune(s[a])) || s[a] == '\'' || s[a] == '"') {
		a++
	}
	for b > a && (isP(rune(s[b-1])) || s[b-1] == '\'' || s[b-1] == '"') {
		b--
	}
	return s[:a], s[a:b], s[b:]
}

// detect command tokens like (up) or (cap,3)
func parseCmd(tok string) (cmd string, n int, ok bool) {
	if len(tok) < 3 || tok[0] != '(' || tok[len(tok)-1] != ')' {
		return "", 0, false
	}
	body := strings.ToLower(strings.TrimSpace(tok[1 : len(tok)-1]))
	parts := strings.Split(body, ",")
	cmd = strings.TrimSpace(parts[0])
	n = 1
	if len(parts) > 1 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && v > 0 {
			n = v
		}
	}
	switch cmd {
	case "hex", "bin", "up", "low", "cap":
		return cmd, n, true
	}
	return "", 0, false
}

func startsVowelOrH(s string) bool {
	if s == "" {
		return false
	}
	r := unicode.ToLower([]rune(s)[0])
	return r == 'a' || r == 'e' || r == 'i' || r == 'o' || r == 'u' || r == 'h'
}

func titleOne(s string) string {
	if s == "" {
		return s
	}
	r := []rune(strings.ToLower(s))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: go run . in.txt out.txt")
		return
	}
	in, out := os.Args[1], os.Args[2]
	b, err := os.ReadFile(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		return
	}

	// 1) tokenize by whitespace, then split off leading/trailing punctuation
	raw := strings.Fields(string(b))
	var toks []string
	for _, w := range raw {
		pre, core, suf := stripEdges(w)
		if pre != "" {
			toks = append(toks, pre)
		}
		if core != "" {
			toks = append(toks, core)
		}
		if suf != "" {
			toks = append(toks, suf)
		}
	}

	// 2) process commands left-to-right, writing into 'outToks'
	var outToks []string
	for i := 0; i < len(toks); i++ {
		// If token begins with '(' but doesn't end with ')', join tokens until we find the closing ')'.
		tok := toks[i]
		var combined string
		if strings.HasPrefix(tok, "(") && !strings.HasSuffix(tok, ")") {
			j := i
			for j+1 < len(toks) && !strings.HasSuffix(toks[j], ")") {
				j++
			}
			// join from i..j (inclusive). If we didn't find a closing ')', fall back to the single token.
			if j >= i {
				combined = strings.Join(toks[i:j+1], " ")
				// advance i to j so the for-loop will continue after the group
				i = j
			} else {
				combined = tok
			}
		} else {
			combined = tok
		}

		if cmd, num, ok := parseCmd(combined); ok {
			switch cmd {
			case "hex":
				if len(outToks) > 0 {
					p, c, s := stripEdges(outToks[len(outToks)-1])
					if v, e := strconv.ParseInt(c, 16, 64); e == nil {
						outToks[len(outToks)-1] = p + strconv.FormatInt(v, 10) + s
					}
				}
			case "bin":
				if len(outToks) > 0 {
					p, c, s := stripEdges(outToks[len(outToks)-1])
					if v, e := strconv.ParseInt(c, 2, 64); e == nil {
						outToks[len(outToks)-1] = p + strconv.FormatInt(v, 10) + s
					}
				}
			case "up", "low", "cap":
				need := num
				j := len(outToks) - 1
				for j >= 0 && need > 0 {
					// skip pure-punctuation tokens
					allP := true
					for _, r := range outToks[j] {
						if !isP(rune(r)) && r != '\'' && r != '"' {
							allP = false
							break
						}
					}
					if allP {
						j--
						continue
					}
					p, c, s := stripEdges(outToks[j])
					if c != "" {
						switch cmd {
						case "up":
							c = strings.ToUpper(c)
						case "low":
							c = strings.ToLower(c)
						case "cap":
							c = titleOne(c)
						}
						outToks[j] = p + c + s
						need--
					}
					j--
				}
			}
			continue // don't emit the command token itself
		}

		// if not a command (or parse failed), emit the original token(s)
		if strings.Contains(combined, " ") {
			parts := strings.Split(combined, " ")
			outToks = append(outToks, parts...)
		} else {
			outToks = append(outToks, combined)
		}
	}

	// 4) a -> an
	for i := 0; i < len(outToks); i++ {
		if strings.ToLower(outToks[i]) == "a" {
			next := -1
			for j := i + 1; j < len(outToks); j++ {
				isAllP := true
				for _, r := range outToks[j] {
					if !isP(rune(r)) && r != '\'' && r != '"' {
						isAllP = false
						break
					}
				}
				if !isAllP {
					next = j
					break
				}
			}
			if next != -1 {
				_, core, _ := stripEdges(outToks[next])
				if startsVowelOrH(core) {
					if outToks[i] == "A" {
						outToks[i] = "An"
					} else {
						outToks[i] = "an"
					}
				}
			}
		}
	}

	// 3) join single-quote groups ' ... '
	var grouped []string
	for i := 0; i < len(outToks); i++ {
		if outToks[i] == "'" {
			j := i + 1
			for j < len(outToks) && outToks[j] != "'" {
				j++
			}
			if j < len(outToks) && outToks[j] == "'" {
				grouped = append(grouped, "'"+strings.Join(outToks[i+1:j], " ")+"'")
				i = j
				continue
			}
		}
		grouped = append(grouped, outToks[i])
	}
	outToks = grouped

	// 5) build final string: attach punctuation tokens to previous word
	var bld strings.Builder
	for i := 0; i < len(outToks); i++ {
		t := outToks[i]
		allP := true
		for _, r := range t {
			if !isP(rune(r)) && r != '\'' && r != '"' {
				allP = false
				break
			}
		}
		if i == 0 {
			bld.WriteString(t)
			continue
		}
		if allP {
			bld.WriteString(t)
			if i+1 < len(outToks) {
				nextAll := true
				for _, r := range outToks[i+1] {
					if !isP(rune(r)) && r != '\'' && r != '"' {
						nextAll = false
						break
					}
				}
				if !nextAll {
					bld.WriteByte(' ')
				}
			}
		} else {
			last := bld.String()
			if len(last) > 0 && last[len(last)-1] != ' ' {
				bld.WriteByte(' ')
			}
			bld.WriteString(t)
		}
	}

	res := bld.String()
	// small cleanups: spaces before punctuation
	res = strings.ReplaceAll(res, " ,", ",")
	res = strings.ReplaceAll(res, " .", ".")
	res = strings.ReplaceAll(res, " !", "!")
	res = strings.ReplaceAll(res, " ?", "?")
	res = strings.ReplaceAll(res, " :", ":")
	res = strings.ReplaceAll(res, " ;", ";")
	for strings.Contains(res, "  ") {
		res = strings.ReplaceAll(res, "  ", " ")
	}
	res = strings.TrimSpace(res)

	if err := os.WriteFile(out, []byte(res), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
	}
}
