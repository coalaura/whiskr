package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type GitHubRepo struct {
	Name          string `json:"name"`
	HtmlURL       string `json:"html_url"`
	Description   string `json:"description"`
	Stargazers    int    `json:"stargazers_count"`
	Forks         int    `json:"forks_count"`
	Visibility    string `json:"visibility"`
	DefaultBranch string `json:"default_branch"`
}

type GitHubTreeItem struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type GitHubTreeResponse struct {
	Truncated bool             `json:"truncated"`
	Tree      []GitHubTreeItem `json:"tree"`
}

type GitHubReadme struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

var IgnoreGithubPaths = []string{
	"node_modules/", "vendor/", ".git/", "dist/", "build/",
	"bin/", "obj/", "out/", ".idea/", ".vscode/", "__pycache__/",
}

func (r *GitHubReadme) AsText() (string, error) {
	if r.Encoding == "base64" {
		content, err := base64.StdEncoding.DecodeString(r.Content)
		if err != nil {
			return "", err
		}

		return string(content), nil
	}

	return r.Content, nil
}

func NewGitHubRequest(ctx context.Context, path string) (*http.Request, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com%s", path), nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	req.Header.Set("Accept", "application/vnd.github+json")

	if env.Tokens.GitHub != "" {
		req.Header.Set("Authorization", "Bearer "+env.Tokens.GitHub)
	}

	return req, nil
}

func GitHubRepositoryJson(ctx context.Context, owner, repo string) (*GitHubRepo, error) {
	req, err := NewGitHubRequest(ctx, fmt.Sprintf("/repos/%s/%s", owner, repo))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var response GitHubRepo

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	if response.Name == "" {
		return nil, errors.New("error getting data")
	}

	if response.Description == "" {
		response.Description = "(none)"
	}

	return &response, nil
}

func GitHubRepositoryReadmeJson(ctx context.Context, owner, repo, branch string) (*GitHubReadme, error) {
	req, err := NewGitHubRequest(ctx, fmt.Sprintf("/repos/%s/%s/readme?ref=%s", owner, repo, branch))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var response GitHubReadme

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func GitHubRepositoryTreeJson(ctx context.Context, owner, repo, branch string) (*GitHubTreeResponse, error) {
	req, err := NewGitHubRequest(ctx, fmt.Sprintf("/repos/%s/%s/git/trees/%s?recursive=1", owner, repo, branch))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var response GitHubTreeResponse

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func RepoOverview(ctx context.Context, arguments *GitHubRepositoryArguments) (string, error) {
	repository, err := GitHubRepositoryJson(ctx, arguments.Owner, arguments.Repo)
	if err != nil {
		return "", err
	}

	var (
		wg sync.WaitGroup

		readmeMarkdown string
		files          []string
		treeTruncated  bool
	)

	// fetch readme
	wg.Go(func() {
		readme, err := GitHubRepositoryReadmeJson(ctx, arguments.Owner, arguments.Repo, repository.DefaultBranch)
		if err != nil {
			log.Warnf("failed to get repository readme: %v\n", err)

			readmeMarkdown = fmt.Sprintf("*Failed to load README: %v*", err)

			return
		}

		markdown, err := readme.AsText()
		if err != nil {
			log.Warnf("failed to decode repository readme: %v\n", err)

			readmeMarkdown = fmt.Sprintf("*Failed to load README: %v*", err)

			return
		}

		readmeMarkdown = markdown
	})

	// fetch contents
	wg.Go(func() {
		tree, err := GitHubRepositoryTreeJson(ctx, arguments.Owner, arguments.Repo, repository.DefaultBranch)
		if err != nil {
			log.Warnf("failed to get repository contents: %v\n", err)

			return
		}

		var validItems []GitHubTreeItem

		for _, item := range tree.Tree {
			if !shouldIgnoreGithubFile(item.Path) {
				validItems = append(validItems, item)
			}
		}

		sort.Slice(validItems, func(i, j int) bool {
			depthI := strings.Count(validItems[i].Path, "/")
			depthJ := strings.Count(validItems[j].Path, "/")

			if depthI == depthJ {
				return validItems[i].Path < validItems[j].Path
			}

			return depthI < depthJ
		})

		if len(validItems) > 256 {
			validItems = validItems[:256]

			treeTruncated = true
		} else if tree.Truncated {
			treeTruncated = true
		}

		for _, item := range validItems {
			if item.Type == "tree" {
				files = append(files, fmt.Sprintf(
					"- [D] [%s](https://github.com/%s/%s/tree/%s/%s)",
					item.Path, arguments.Owner, arguments.Repo, repository.DefaultBranch, item.Path,
				))
			} else { // "blob"
				files = append(files, fmt.Sprintf(
					"- [F] [%s](https://raw.githubusercontent.com/%s/%s/refs/heads/%s/%s)",
					item.Path, arguments.Owner, arguments.Repo, repository.DefaultBranch, item.Path,
				))
			}
		}
	})

	// wait and combine results
	wg.Wait()

	buf := GetFreeBuffer()
	defer pool.Put(buf)

	fmt.Fprintf(buf, "### %s (%s)\n", repository.Name, repository.Visibility)
	fmt.Fprintf(buf, "- URL: %s\n", repository.HtmlURL)
	fmt.Fprintf(buf, "- Description: %s\n", strings.ReplaceAll(repository.Description, "\n", " "))
	fmt.Fprintf(buf, "- Default branch: %s\n", repository.DefaultBranch)
	fmt.Fprintf(buf, "- Stars: %d | Forks: %d\n", repository.Stargazers, repository.Forks)

	buf.WriteString("\n### Repository Structure\n")

	if len(files) == 0 {
		buf.WriteString("*No entries or insufficient permissions.*\n")
	} else {
		for _, file := range files {
			fmt.Fprintf(buf, "%s\n", file)
		}

		if treeTruncated {
			buf.WriteString("\n*... (repository tree truncated to save context) ...*\n")
		}
	}

	buf.WriteString("\n### README\n")
	buf.WriteString(readmeMarkdown)

	return buf.String(), nil
}

func shouldIgnoreGithubFile(path string) bool {
	for _, ignore := range IgnoreGithubPaths {
		if strings.HasPrefix(path, ignore) || strings.Contains(path, "/"+ignore) {
			return true
		}
	}

	return false
}
