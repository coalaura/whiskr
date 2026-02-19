package main

import (
	"bufio"
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

type TreeNode struct {
	TokenID  int
	Children map[byte]*TreeNode
}

type Tokenizer struct {
	Root *TreeNode
}

func NewTreeNode() *TreeNode {
	return &TreeNode{
		TokenID:  -1,
		Children: make(map[byte]*TreeNode),
	}
}

func (n *TreeNode) Insert(token []byte, id int) {
	curr := n

	for _, b := range token {
		if _, ok := curr.Children[b]; !ok {
			curr.Children[b] = NewTreeNode()
		}

		curr = curr.Children[b]
	}

	curr.TokenID = id
}

func LoadTokenizer(url string) (*Tokenizer, error) {
	err := PreloadVocabulary(url, TikTokenPath)
	if err != nil {
		return nil, err
	}

	log.Println("Loading tokenizer...")

	vocabulary, err := LoadVocabulary(TikTokenPath)
	if err != nil {
		return nil, err
	}

	root := NewTreeNode()

	for tokenStr, id := range vocabulary {
		root.Insert([]byte(tokenStr), id)
	}

	return &Tokenizer{
		Root: root,
	}, nil
}

func (t *Tokenizer) Encode(text string) []int {
	var (
		index  int
		tokens []int
	)

	input := []byte(text)

	for index < len(input) {
		bestMatchLength := 0
		bestMatchID := -1

		currNode := t.Root

		for i := index; i < len(input); i++ {
			b := input[i]

			childNode, exists := currNode.Children[b]
			if !exists {
				break
			}

			currNode = childNode

			if currNode.TokenID != -1 {
				bestMatchID = currNode.TokenID
				bestMatchLength = (i - index) + 1
			}
		}

		// should not be possible
		if bestMatchLength == 0 {
			bestMatchLength = 1
		}

		if bestMatchID != -1 {
			tokens = append(tokens, bestMatchID)
		}

		index += bestMatchLength
	}

	return tokens
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
