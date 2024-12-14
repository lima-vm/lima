package limatmpl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/containerd/containerd/identifiers"
	"github.com/lima-vm/lima/pkg/ioutilx"
	"github.com/lima-vm/lima/pkg/templatestore"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type Template struct {
	Name    string
	Locator string
	Bytes   []byte

	algorithm digest.Algorithm
	digest    string
}

const yBytesLimit = 4 * 1024 * 1024 // 4MiB

// Only sha256, sha384, and sha512 are actually available but we reserve all lowercase and digit strings.
// Note that only lowercase hex digits are accepted.
var digestSuffixRegex = regexp.MustCompile(`^(.+)@(?:([a-z0-9]+):)?([a-f0-9]+)$`)

// splitOffDigest splits off an optional @algorithm:digest suffix from the locator.
func (tmpl *Template) splitOffDigest() error {
	matches := digestSuffixRegex.FindStringSubmatch(tmpl.Locator)
	if matches != nil {
		tmpl.algorithm = digest.Algorithm(matches[2])
		if tmpl.algorithm == "" {
			tmpl.algorithm = digest.SHA256
		}
		if !tmpl.algorithm.Available() {
			return fmt.Errorf("locator %q uses unavailable digest algorithm", tmpl.Locator)
		}
		tmpl.digest = matches[3]
		if len(tmpl.digest) < 7 {
			return fmt.Errorf("locator %q digest has fewer than 7 hex digits", tmpl.Locator)
		}
		tmpl.Locator = matches[1]
	}
	return nil
}

// Read fetches the content pointed at by a template locator. If the locator has an optional
// digest suffix, then the digest must match, or Read will return an error.
func Read(ctx context.Context, name, locator string) (*Template, error) {
	var err error

	tmpl := &Template{
		Name:    name,
		Locator: locator,
	}
	if err = tmpl.splitOffDigest(); err != nil {
		return nil, err
	}

	isTemplateURL, templateURL := SeemsTemplateURL(tmpl.Locator)
	switch {
	case isTemplateURL:
		// No need to use SecureJoin here. https://github.com/lima-vm/lima/pull/805#discussion_r853411702
		templateName := filepath.Join(templateURL.Host, templateURL.Path)
		logrus.Debugf("interpreting argument %q as a template name %q", tmpl.Locator, templateName)
		if tmpl.Name == "" {
			// e.g., templateName = "deprecated/centos-7" , tmpl.Name = "centos-7"
			tmpl.Name = filepath.Base(templateName)
		}
		tmpl.Bytes, err = templatestore.Read(templateName)
		if err != nil {
			return nil, err
		}
	case SeemsHTTPURL(tmpl.Locator):
		if tmpl.Name == "" {
			tmpl.Name, err = InstNameFromURL(tmpl.Locator)
			if err != nil {
				return nil, err
			}
		}
		logrus.Debugf("interpreting argument %q as a http url for instance %q", tmpl.Locator, tmpl.Name)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, tmpl.Locator, http.NoBody)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		tmpl.Bytes, err = ioutilx.ReadAtMaximum(resp.Body, yBytesLimit)
		if err != nil {
			return nil, err
		}
	case SeemsFileURL(tmpl.Locator):
		if tmpl.Name == "" {
			tmpl.Name, err = InstNameFromURL(tmpl.Locator)
			if err != nil {
				return nil, err
			}
		}
		logrus.Debugf("interpreting argument %q as a file url for instance %q", tmpl.Locator, tmpl.Name)
		r, err := os.Open(strings.TrimPrefix(tmpl.Locator, "file://"))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		tmpl.Bytes, err = ioutilx.ReadAtMaximum(r, yBytesLimit)
		if err != nil {
			return nil, err
		}
	case SeemsYAMLPath(tmpl.Locator):
		if tmpl.Name == "" {
			tmpl.Name, err = InstNameFromYAMLPath(tmpl.Locator)
			if err != nil {
				return nil, err
			}
		}
		logrus.Debugf("interpreting argument %q as a file path for instance %q", tmpl.Locator, tmpl.Name)
		r, err := os.Open(tmpl.Locator)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		tmpl.Bytes, err = ioutilx.ReadAtMaximum(r, yBytesLimit)
		if err != nil {
			return nil, err
		}
	case tmpl.Locator == "-":
		tmpl.Bytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("unexpected error reading stdin: %w", err)
		}
	}

	if tmpl.digest != "" {
		actualDigest := digest.Algorithm(tmpl.algorithm).FromBytes(tmpl.Bytes).Encoded()
		if len(tmpl.digest) < len(actualDigest) {
			actualDigest = actualDigest[:len(tmpl.digest)]
		}
		if actualDigest != tmpl.digest {
			return nil, fmt.Errorf("locator %q digest doesn't match content digest %q", locator, actualDigest)
		}
	}
	return tmpl, nil
}

func SeemsTemplateURL(arg string) (bool, *url.URL) {
	u, err := url.Parse(arg)
	if err != nil {
		return false, u
	}
	return u.Scheme == "template", u
}

func SeemsHTTPURL(arg string) bool {
	u, err := url.Parse(arg)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return true
}

func SeemsFileURL(arg string) bool {
	u, err := url.Parse(arg)
	if err != nil {
		return false
	}
	return u.Scheme == "file"
}

func SeemsYAMLPath(arg string) bool {
	if strings.Contains(arg, "/") {
		return true
	}
	lower := strings.ToLower(arg)
	return strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")
}

func InstNameFromURL(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	return InstNameFromYAMLPath(path.Base(u.Path))
}

func InstNameFromYAMLPath(yamlPath string) (string, error) {
	s := strings.ToLower(filepath.Base(yamlPath))
	s = strings.TrimSuffix(strings.TrimSuffix(s, ".yml"), ".yaml")
	// "." is allowed in instance names, but replaced to "-" for hostnames.
	// e.g., yaml: "ubuntu-24.04.yaml" , instance name: "ubuntu-24.04", hostname: "lima-ubuntu-24-04"
	if err := identifiers.Validate(s); err != nil {
		return "", fmt.Errorf("filename %q is invalid: %w", yamlPath, err)
	}
	return s, nil
}
