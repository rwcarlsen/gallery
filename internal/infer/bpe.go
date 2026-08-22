package infer

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	sotToken = 49406
	eotToken = 49407
	ctxLen   = 77
)

type Tokenizer struct {
	encoder  map[string]int
	bpeRanks map[[2]string]int
	byteEnc  [256]rune
	pat      *regexp.Regexp
	cache    map[string]string
}

func LoadTokenizer(path string) (*Tokenizer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r *bufio.Reader
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = bufio.NewReader(gz)
	} else {
		r = bufio.NewReader(f)
	}

	var merges [][2]string
	first := true
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			line = strings.TrimRight(line, "\r\n")
			if first {
				first = false
				continue
			}
			if line == "" {
				continue
			}
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				merges = append(merges, [2]string{parts[0], parts[1]})
			}
		}
		if err != nil {
			break
		}
	}
	if len(merges) > 48894 {
		merges = merges[:48894]
	}

	t := &Tokenizer{
		encoder:  map[string]int{},
		bpeRanks: map[[2]string]int{},
		cache:    map[string]string{},
		pat:      regexp.MustCompile(`(?i)<\|startoftext\|>|<\|endoftext\|>|'s|'t|'re|'ve|'m|'ll|'d|[^\s]+`),
	}
	for i := 0; i < 256; i++ {
		t.byteEnc[i] = bytesToUnicode(byte(i))
	}
	vocab := make([]string, 0, 50000)
	for i := 0; i < 256; i++ {
		vocab = append(vocab, string(t.byteEnc[i]))
	}
	for i := 0; i < 256; i++ {
		vocab = append(vocab, string(t.byteEnc[i])+"</w>")
	}
	for _, m := range merges {
		t.bpeRanks[m] = len(t.bpeRanks)
		vocab = append(vocab, m[0]+m[1])
	}
	vocab = append(vocab, "<|startoftext|>", "<|endoftext|>")
	for i, v := range vocab {
		t.encoder[v] = i
	}
	return t, nil
}

func bytesToUnicode(b byte) rune {
	if (b >= '!' && b <= '~') || (b >= 0xA1 && b <= 0xAC) || (b >= 0xAE && b <= 0xFF) {
		return rune(b)
	}
	n := 0
	for i := 0; i < 256; i++ {
		bb := byte(i)
		ok := (bb >= '!' && bb <= '~') || (bb >= 0xA1 && bb <= 0xAC) || (bb >= 0xAE && bb <= 0xFF)
		if !ok {
			if bb == b {
				return rune(256 + n)
			}
			n++
		}
	}
	return rune(b)
}

func (t *Tokenizer) Encode(text string) ([]int64, error) {
	if t == nil {
		return nil, fmt.Errorf("no tokenizer")
	}
	text = strings.TrimSpace(strings.ToLower(text))
	ids := make([]int64, 0, ctxLen)
	ids = append(ids, sotToken)
	for _, tok := range t.pat.FindAllString(text, -1) {
		var b strings.Builder
		for i := 0; i < len(tok); i++ {
			b.WriteRune(t.byteEnc[tok[i]])
		}
		for _, piece := range strings.Split(t.bpe(b.String()), " ") {
			if piece == "" {
				continue
			}
			id, ok := t.encoder[piece]
			if !ok {
				continue
			}
			if len(ids) >= ctxLen-1 {
				break
			}
			ids = append(ids, int64(id))
		}
		if len(ids) >= ctxLen-1 {
			break
		}
	}
	ids = append(ids, eotToken)
	out := make([]int64, ctxLen)
	copy(out, ids)
	if len(ids) < ctxLen {
		// already zero-padded
	} else {
		out[ctxLen-1] = eotToken
	}
	return out, nil
}

func (t *Tokenizer) bpe(token string) string {
	if v, ok := t.cache[token]; ok {
		return v
	}
	word := make([]string, 0, len(token))
	rs := []rune(token)
	for i, r := range rs {
		if i == len(rs)-1 {
			word = append(word, string(r)+"</w>")
		} else {
			word = append(word, string(r))
		}
	}
	for {
		if len(word) == 1 {
			break
		}
		best := -1
		var pair [2]string
		for i := 0; i < len(word)-1; i++ {
			p := [2]string{word[i], word[i+1]}
			rank, ok := t.bpeRanks[p]
			if ok && (best < 0 || rank < best) {
				best = rank
				pair = p
			}
		}
		if best < 0 {
			break
		}
		var next []string
		for i := 0; i < len(word); {
			if i < len(word)-1 && word[i] == pair[0] && word[i+1] == pair[1] {
				next = append(next, pair[0]+pair[1])
				i += 2
			} else {
				next = append(next, word[i])
				i++
			}
		}
		word = next
	}
	out := strings.Join(word, " ")
	if len(t.cache) < 10000 {
		t.cache[token] = out
	}
	return out
}
