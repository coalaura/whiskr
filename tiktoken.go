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

	if len(input) == 0 {
		return 0
	}

	if len(input) == 1 {
		return 1
	}

	n := len(input)

	prev := make([]int, n)
	next := make([]int, n)

	for i := range n {
		prev[i] = i - 1
		next[i] = i + 1
	}

	next[n-1] = -1

	length := make([]int, n)

	for i := range n {
		length[i] = 1
	}

	for {
		bestRank := int((^uint(0)) >> 1) // MaxInt
		bestIdx := -1

		var curr int

		for curr != -1 {
			nxt := next[curr]
			if nxt == -1 {
				break
			}

			pairBytes := input[curr : curr+length[curr]+length[nxt]]

			if rank, exists := t.Ranks[string(pairBytes)]; exists {
				if rank < bestRank {
					bestRank = rank
					bestIdx = curr
				}
			}

			curr = nxt
		}

		if bestIdx == -1 {
			break
		}

		nxt := next[bestIdx]

		length[bestIdx] += length[nxt]

		next[bestIdx] = next[nxt]

		if next[nxt] != -1 {
			prev[next[nxt]] = bestIdx
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
