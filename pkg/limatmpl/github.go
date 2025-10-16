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
	"strings"
)

const defaultFilename = ".lima.yaml"

// transformGitHubURL transforms a github: URL to a raw.githubusercontent.com URL.
// Input format: ORG/REPO[/PATH][@BRANCH]
//
// If REPO is missing, it will be set the same as ORG.
// If BRANCH is missing, it will be queried from GitHub.
// If PATH filename has no extension, it will get .yaml.
// If PATH is just a directory (trailing slash), it will be set to .lima.yaml
// IF FILE is .lima.yaml and contents looks like a symlink, it will be replaced by the symlink target.
func transformGitHubURL(ctx context.Context, input string) (string, error) {
	input, origBranch, _ := strings.Cut(input, "@")

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
	filePath := strings.Join(parts[2:], "/")

	if filePath == "" {
		filePath = defaultFilename
	} else {
		// If path ends with / then it's a directory, so append .lima
		if strings.HasSuffix(filePath, "/") {
			filePath += defaultFilename
		}

		// If the filename (excluding first char for hidden files) has no extension, add .yaml
		filename := path.Base(filePath)
		if !strings.Contains(filename[1:], ".") {
			filePath += ".yaml"
		}
	}

	// Query default branch if no branch was specified
	branch := origBranch
	if branch == "" {
		var err error
		branch, err = getGitHubDefaultBranch(ctx, org, repo)
		if err != nil {
			return "", fmt.Errorf("failed to get default branch for %s/%s: %w", org, repo, err)
		}
	}

	// If filename is .lima.yaml, check if it's a symlink/redirect to another file
	if path.Base(filePath) == defaultFilename {
		return resolveGitHubSymlink(ctx, org, repo, branch, filePath, origBranch)
	}
	return githubUserContentURL(org, repo, branch, filePath), nil
}

func githubUserContentURL(org, repo, branch, filePath string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", org, repo, branch, filePath)
}

func getGitHubUserContent(ctx context.Context, org, repo, branch, filePath string) (*http.Response, error) {
	url := githubUserContentURL(org, repo, branch, filePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "lima")
	return http.DefaultClient.Do(req)
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
// If the file contains a single line without newline, space, or colon then it's treated as a path to the actual file.
// Returns a URL to the redirect path if found, or a URL for original path otherwise.
func resolveGitHubSymlink(ctx context.Context, org, repo, branch, filePath, origBranch string) (string, error) {
	resp, err := getGitHubUserContent(ctx, org, repo, branch, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file: %w", err)
	}
	defer resp.Body.Close()

	// Special rule for branch/tag propagation for github:ORG// requests.
	if resp.StatusCode == http.StatusNotFound && repo == org {
		defaultBranch, err := getGitHubDefaultBranch(ctx, org, repo)
		if err == nil {
			return resolveGitHubRedirect(ctx, org, repo, defaultBranch, filePath, branch)
		}
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("file %q not found or inaccessible: status %d", resp.Request.URL, resp.StatusCode)
	}

	// Read first 1KB to check the file content
	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed to read %q content: %w", resp.Request.URL, err)
	}
	content := string(buf[:n])

	// Symlink can also be a github: redirect if we are in a github:ORG// repo.
	if repo == org && strings.HasPrefix(content, "github:") {
		return validateGitHubRedirect(content, org, origBranch, resp.Request.URL.String())
	}

	// A symlink must be a single line (without trailing newline), no spaces, no colons
	if !(content == "" || strings.ContainsAny(content, "\n :")) {
		// symlink is relative to the directory of filePath
		filePath = path.Join(path.Dir(filePath), content)
	}
	return githubUserContentURL(org, repo, branch, filePath), nil
}

// resolveGitHubRedirect checks if a file at the given path is a github: URL to another file within the same repo.
// Returns the URL, or an error if the file doesn't exist, or doesn't start with github:ORG.
func resolveGitHubRedirect(ctx context.Context, org, repo, defaultBranch, filePath, origBranch string) (string, error) {
	// Refetch the filepath from the defaultBranch
	resp, err := getGitHubUserContent(ctx, org, repo, defaultBranch, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("file %q not found or inaccessible: status %d", resp.Request.URL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read %q content: %w", resp.Request.URL, err)
	}
	return validateGitHubRedirect(string(body), org, origBranch, resp.Request.URL.String())
}

func validateGitHubRedirect(body, org, origBranch, url string) (string, error) {
	redirect, _, _ := strings.Cut(body, "\n")
	redirect = strings.TrimSpace(redirect)

	if !strings.HasPrefix(redirect, "github:"+org+"/") {
		return "", fmt.Errorf(`redirect %q is not a "github:%s" URL (from %q)`, redirect, org, url)
	}
	if strings.ContainsRune(redirect, '@') {
		return "", fmt.Errorf("redirect %q must not include a branch/tag/sha (from %q)", redirect, url)
	}
	// If the origBranch is empty, then we need to look up the default branch in the redirect
	if origBranch != "" {
		redirect += "@" + origBranch
	}
	return redirect, nil
}
