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

type GitHubContent struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type GitHubReadme struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
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

func GitHubRepositoryContentsJson(ctx context.Context, owner, repo, branch string) ([]GitHubContent, error) {
	req, err := NewGitHubRequest(ctx, fmt.Sprintf("/repos/%s/%s/contents?ref=%s", owner, repo, branch))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var response []GitHubContent

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func RepoOverview(ctx context.Context, arguments GitHubRepositoryArguments) (string, error) {
	repository, err := GitHubRepositoryJson(ctx, arguments.Owner, arguments.Repo)
	if err != nil {
		return "", err
	}

	var (
		wg sync.WaitGroup

		readmeMarkdown string
		directories    []string
		files          []string
	)

	// fetch readme
	wg.Add(1)

	go func() {
		defer wg.Done()

		readme, err := GitHubRepositoryReadmeJson(ctx, arguments.Owner, arguments.Repo, repository.DefaultBranch)
		if err != nil {
			log.Warnf("failed to get repository readme: %v\n", err)

			return
		}

		markdown, err := readme.AsText()
		if err != nil {
			log.Warnf("failed to decode repository readme: %v\n", err)

			return
		}

		readmeMarkdown = markdown
	}()

	// fetch contents
	wg.Add(1)

	go func() {
		defer wg.Done()

		contents, err := GitHubRepositoryContentsJson(ctx, arguments.Owner, arguments.Repo, repository.DefaultBranch)
		if err != nil {
			log.Warnf("failed to get repository contents: %v\n", err)

			return
		}

		for _, content := range contents {
			switch content.Type {
			case "dir":
				directories = append(directories, fmt.Sprintf(
					"[%s](https://github.com/%s/%s/tree/%s/%s)",
					content.Name,
					arguments.Owner,
					arguments.Repo,
					repository.DefaultBranch,
					content.Name,
				))
			case "file":
				files = append(files, fmt.Sprintf(
					"[%s](https://raw.githubusercontent.com/%s/%s/refs/heads/%s/%s)",
					content.Name,
					arguments.Owner,
					arguments.Repo,
					repository.DefaultBranch,
					content.Name,
				))
			}
		}

		sort.Strings(directories)
		sort.Strings(files)
	}()

	// wait and combine results
	wg.Wait()

	var builder strings.Builder

	fmt.Fprintf(&builder, "### %s (%s)\n", repository.Name, repository.Visibility)
	fmt.Fprintf(&builder, "- URL: %s\n", repository.HtmlURL)
	fmt.Fprintf(&builder, "- Description: %s\n", strings.ReplaceAll(repository.Description, "\n", " "))
	fmt.Fprintf(&builder, "- Default branch: %s\n", repository.DefaultBranch)
	fmt.Fprintf(&builder, "- Stars: %d | Forks: %d\n", repository.Stargazers, repository.Forks)

	builder.WriteString("\n### Top-level files and directories\n")

	if len(directories) == 0 && len(files) == 0 {
		builder.WriteString("*No entries or insufficient permissions.*\n")
	} else {
		for _, directory := range directories {
			fmt.Fprintf(&builder, "- [D] %s\n", directory)
		}

		for _, file := range files {
			fmt.Fprintf(&builder, "- [F] %s\n", file)
		}
	}

	builder.WriteString("\n### README\n")

	if readmeMarkdown == "" {
		builder.WriteString("*No README found or could not load.*\n")
	} else {
		builder.WriteString(readmeMarkdown)
	}

	return builder.String(), nil
}
