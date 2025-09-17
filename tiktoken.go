package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const TikTokenSource = "https://openaipublic.blob.core.windows.net/encodings/o200k_base.tiktoken"

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
	log.Println("Loading tokenizer...")

	vocabulary, err := LoadVocabulary(url)
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

func LoadVocabulary(url string) (map[string]int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	vocab := make(map[string]int)

	scanner := bufio.NewScanner(resp.Body)

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
