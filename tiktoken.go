package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	TikTokenSource = "https://openaipublic.blob.core.windows.net/encodings/o200k_base.tiktoken"
	TikTokenPath   = "vocabulary.tiktoken"
)

type Tokenizer struct {
	Ranks map[string]int
}

type mergeCandidate struct {
	rank     int
	idx      int
	lenLeft  int
	lenRight int
}

type candidateHeap []mergeCandidate

func LoadTokenizer(url string) (*Tokenizer, error) {
	err := PreloadVocabulary(url, TikTokenPath)
	if err != nil {
		return nil, err
	}

	log.Println("Loading tokenizer...")

	file, err := os.OpenFile(TikTokenPath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	ranks := make(map[string]int, 200000)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Bytes()

		parts := bytes.SplitN(line, []byte(" "), 2)
		if len(parts) != 2 {
			continue
		}

		decodedLen := base64.StdEncoding.DecodedLen(len(parts[0]))

		decoded := make([]byte, decodedLen)

		n, err := base64.StdEncoding.Decode(decoded, parts[0])
		if err != nil {
			return nil, err
		}

		id, err := strconv.Atoi(string(parts[1]))
		if err != nil {
			return nil, err
		}

		ranks[string(decoded[:n])] = id
	}

	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	return &Tokenizer{Ranks: ranks}, nil
}

func (t *Tokenizer) CountTokens(text string) int {
	input := []byte(text)
	n := len(input)

	if n <= 1 {
		return n
	}

	prev := make([]int, n)
	next := make([]int, n)
	length := make([]int, n)

	for i := range n {
		prev[i] = i - 1
		next[i] = i + 1
		length[i] = 1
	}

	next[n-1] = -1

	pq := make(candidateHeap, 0, n)

	for i := 0; i < n-1; i++ {
		pairBytes := input[i : i+2]

		if rank, exists := t.Ranks[string(pairBytes)]; exists {
			pq.push(mergeCandidate{
				rank:     rank,
				idx:      i,
				lenLeft:  1,
				lenRight: 1,
			})
		}
	}

	for len(pq) > 0 {
		best := pq.pop()

		curr := best.idx
		nxt := next[curr]

		if length[curr] == 0 || nxt == -1 || length[curr] != best.lenLeft || length[nxt] != best.lenRight {
			continue
		}

		length[curr] += length[nxt]
		length[nxt] = 0

		next[curr] = next[nxt]

		if next[nxt] != -1 {
			prev[next[nxt]] = curr
		}

		prv := prev[curr]
		if prv != -1 {
			pairBytes := input[prv : prv+length[prv]+length[curr]]
			if rank, exists := t.Ranks[string(pairBytes)]; exists {
				pq.push(mergeCandidate{
					rank:     rank,
					idx:      prv,
					lenLeft:  length[prv],
					lenRight: length[curr],
				})
			}
		}

		newNxt := next[curr]
		if newNxt != -1 {
			pairBytes := input[curr : curr+length[curr]+length[newNxt]]
			if rank, exists := t.Ranks[string(pairBytes)]; exists {
				pq.push(mergeCandidate{
					rank:     rank,
					idx:      curr,
					lenLeft:  length[curr],
					lenRight: length[newNxt],
				})
			}
		}
	}

	var (
		tokenCount int
		curr       int
	)

	for curr != -1 {
		tokenCount++
		curr = next[curr]
	}

	return tokenCount
}

func (h *candidateHeap) push(c mergeCandidate) {
	*h = append(*h, c)

	h.up(len(*h) - 1)
}

func (h *candidateHeap) pop() mergeCandidate {
	old := *h

	n := len(old) - 1

	c := old[0]

	old[0] = old[n]
	*h = old[:n]

	h.down(0, n)

	return c
}

func (h candidateHeap) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || h[j].rank >= h[i].rank {
			break
		}

		h[i], h[j] = h[j], h[i]

		j = i
	}
}

func (h candidateHeap) down(i0, n int) {
	i := i0

	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 {
			break
		}

		j := j1
		if j2 := j1 + 1; j2 < n && h[j2].rank < h[j1].rank {
			j = j2
		}

		if h[j].rank >= h[i].rank {
			break
		}

		h[i], h[j] = h[j], h[i]

		i = j
	}
}

func PreloadVocabulary(url, path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	log.Println("Downloading tokenizer...")

	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func LoadVocabulary(path string) (map[string]int, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	vocab := make(map[string]int)

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), " ", 2)

		if len(parts) != 2 {
			continue
		}

		decoded, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, fmt.Errorf("failed to decode token '%s': %w", parts[0], err)
		}

		id, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse token ID '%s': %w", parts[1], err)
		}

		vocab[string(decoded)] = id
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return vocab, nil
}
