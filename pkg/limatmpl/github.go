// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatmpl

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

// transformGitHubURL transforms a github: URL to a raw.githubusercontent.com URL.
// Input format: [ORG[/REPO]][/PATH][@BRANCH]
//
// If REPO is omitted, it defaults to the same value as ORG.
//
// When no PATH is specified, it uses .lima.yaml from the repository root.
// Files lima.yaml and .lima.yaml are checked if their content looks like a symlink: not YAML
// (no newlines, doesn't start with '{', and doesn't match YAML key pattern). In that case the line
// is treated as the path to the actual template file.
//
// Examples:
//   - github:lima-vm                           -> .lima.yaml (or path from .lima.yaml if it's a symlink)
//   - github:lima-vm//templates                -> lima-vm/lima-vm/master/templates/lima.yaml
//   - github:lima-vm/lima                      -> lima/master/.lima.yaml (or path from .lima.yaml)
//   - github:lima-vm/lima/examples             -> lima/master/examples/lima.yaml
//   - github:lima-vm/lima@v1.0.0               -> lima/v1.0.0/.lima.yaml (or path from .lima.yaml)
//   - github:lima-vm/lima/examples/docker.yaml -> lima/master/examples/docker.yaml
func transformGitHubURL(ctx context.Context, input string) (string, error) {
	// Check for explicit branch specification with @ at the end
	var branch string
	if idx := strings.LastIndex(input, "@"); idx != -1 {
		branch = input[idx+1:]
		input = input[:idx]
	}

	parts := strings.Split(input, "/")
	for len(parts) < 2 {
		parts = append(parts, "")
	}

	org := parts[0]
	if org == "" {
		return "", fmt.Errorf("github: URL must contain at least an ORG, got %q", input)
	}

	// If REPO is omitted (github:ORG or github:ORG//PATH), default it to ORG
	repo := cmp.Or(parts[1], org)
	pathPart := strings.Join(parts[2:], "/")

	if pathPart == "" {
		pathPart = ".lima.yaml"
	} else {
		// If path ends with /, it's a directory, so append lima
		if strings.HasSuffix(pathPart, "/") {
			pathPart += "lima"
		}

		// If the filename has no extension, add .yaml
		filename := path.Base(pathPart)
		if !strings.Contains(filename, ".") {
			pathPart += ".yaml"
		}
	}

	// Query default branch if no branch was specified
	if branch == "" {
		var err error
		branch, err = getGitHubDefaultBranch(ctx, org, repo)
		if err != nil {
			return "", fmt.Errorf("failed to get default branch for %s/%s: %w", org, repo, err)
		}
	}

	// If filename is .lima.yaml or lima.yaml, check if it's a symlink/redirect to another file
	if strings.TrimPrefix(path.Base(pathPart), ".") == "lima.yaml" {
		if redirectPath, err := resolveGitHubSymlink(ctx, org, repo, branch, pathPart); err == nil {
			pathPart = redirectPath
		}
	}

	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", org, repo, branch, pathPart), nil
}

// getGitHubDefaultBranch queries the GitHub API to get the default branch for a repository.
func getGitHubDefaultBranch(ctx context.Context, org, repo string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", org, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "lima")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Check for GitHub token in environment for authenticated requests (higher rate limit)
	token := cmp.Or(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN"))
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query GitHub API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read GitHub API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var repoData struct {
		DefaultBranch string `json:"default_branch"`
	}

	if err := json.Unmarshal(body, &repoData); err != nil {
		return "", fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	if repoData.DefaultBranch == "" {
		return "", fmt.Errorf("repository %s/%s has no default branch", org, repo)
	}

	return repoData.DefaultBranch, nil
}

// resolveGitHubSymlink checks if a file at the given path is a symlink/redirect to another file.
// If the file contains a single line without YAML content, it's treated as a path to the actual file.
// Returns the redirect path if found, or the original path otherwise.
func resolveGitHubSymlink(ctx context.Context, org, repo, branch, filePath string) (string, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", org, repo, branch, filePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "lima")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("file not found or inaccessible: status %d", resp.StatusCode)
	}

	// Read first 1KB to check the file content
	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed to read file content: %w", err)
	}
	content := string(buf[:n])

	if LooksLikeSymlink(content) {
		redirectPath := strings.TrimSpace(content)
		if redirectPath != "" {
			// redirectPath is relative to the directory of filePath
			return path.Join(path.Dir(filePath), redirectPath), nil
		}
	}
	return filePath, nil
}

// LooksLikeSymlink determines if the given content looks like a symlink.
func LooksLikeSymlink(content string) bool {
	if content == "" {
		return false
	}
	if strings.Contains(content, "\n") {
		return false
	}
	// Check for YAML flow style (starts with '{')
	if strings.HasPrefix(strings.TrimSpace(content), "{") {
		return false
	}
	// Check for YAML key pattern: non-whitespace followed by colon and space
	yamlKeyPattern := regexp.MustCompile(`^\S+:\s`)
	return !yamlKeyPattern.MatchString(content)
}
