// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

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
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/identifiers"
	"github.com/lima-vm/lima/v2/pkg/ioutilx"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/templatestore"
)

const yBytesLimit = 4 * 1024 * 1024 // 4MiB

func Read(ctx context.Context, name, locator string) (*Template, error) {
	var err error
	tmpl := &Template{
		Name:    name,
		Locator: locator,
	}

	if imageTemplate(tmpl, locator) {
		return tmpl, nil
	}

	isTemplateURL, templateName := SeemsTemplateURL(locator)
	switch {
	case isTemplateURL:
		logrus.Debugf("interpreting argument %q as a template name %q", locator, templateName)
		if tmpl.Name == "" {
			// e.g., templateName = "deprecated/centos-7.yaml" , tmpl.Name = "centos-7"
			tmpl.Name, err = InstNameFromYAMLPath(templateName)
			if err != nil {
				return nil, err
			}
		}
		tmpl.Bytes, err = templatestore.Read(templateName)
		if err != nil {
			return nil, err
		}
	case SeemsHTTPURL(locator):
		if tmpl.Name == "" {
			tmpl.Name, err = InstNameFromURL(locator)
			if err != nil {
				return nil, err
			}
		}
		logrus.Debugf("interpreting argument %q as a http url for instance %q", locator, tmpl.Name)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, locator, http.NoBody)
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
	case SeemsFileURL(locator):
		if tmpl.Name == "" {
			tmpl.Name, err = InstNameFromURL(locator)
			if err != nil {
				return nil, err
			}
		}
		logrus.Debugf("interpreting argument %q as a file URL for instance %q", locator, tmpl.Name)
		filePath := strings.TrimPrefix(locator, "file://")
		if !filepath.IsAbs(filePath) {
			return nil, fmt.Errorf("file URL %q is not an absolute path", locator)
		}
		r, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		tmpl.Bytes, err = ioutilx.ReadAtMaximum(r, yBytesLimit)
		if err != nil {
			return nil, err
		}
	case locator == "-":
		tmpl.Bytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("unexpected error reading stdin: %w", err)
		}
	default:
		if tmpl.Name == "" {
			tmpl.Name, err = InstNameFromYAMLPath(locator)
			if err != nil {
				return nil, err
			}
		}
		logrus.Debugf("interpreting argument %q as a file path for instance %q", locator, tmpl.Name)
		if locator, err = filepath.Abs(locator); err != nil {
			return nil, err
		}
		tmpl.Locator = locator
		r, err := os.Open(locator)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		tmpl.Bytes, err = ioutilx.ReadAtMaximum(r, yBytesLimit)
		if err != nil {
			return nil, err
		}
	}
	// The only reason not to call tmpl.UseAbsLocators() here is that `limactl tmpl copy --verbatim …`
	// should create an unmodified copy of the template.
	return tmpl, nil
}

// Locators with an image file format extension, optionally followed by a compression method.
// This regex is also used to remove the file format suffix from the instance name.
var imageURLRegex = regexp.MustCompile(`\.(img|qcow2|raw|iso)(\.(gz|xz|bz2|zstd))?$`)

// Image architecture will be guessed based on the presence of arch keywords.
var archKeywords = map[string]limatype.Arch{
	"aarch64": limatype.AARCH64,
	"amd64":   limatype.X8664,
	"arm64":   limatype.AARCH64,
	"armhf":   limatype.ARMV7L,
	"armv7l":  limatype.ARMV7L,
	"ppc64el": limatype.PPC64LE,
	"ppc64le": limatype.PPC64LE,
	"riscv64": limatype.RISCV64,
	"s390x":   limatype.S390X,
	"x86_64":  limatype.X8664,
}

// These generic tags will be stripped from an image name before turning it into an instance name.
var genericTags = []string{
	"base",         // Fedora, Rocky
	"cloud",        // Fedora, openSUSE
	"cloudimg",     // Ubuntu, Arch
	"cloudinit",    // Alpine
	"daily",        // Debian
	"default",      // Gentoo
	"generic",      // Fedora
	"genericcloud", // CentOS, Debian, Rocky, Alma
	"kvm",          // Oracle
	"latest",       // Gentoo, CentOS, Rocky, Alma
	"linux",        // Arch
	"minimal",      // openSUSE
	"openstack",    // Gentoo
	"server",       // Ubuntu
	"std",          // Alpine-Lima
	"stream",       // CentOS
	"uefi",         // Alpine
	"vm",           // openSUSE
}

// imageTemplate checks if the locator specifies an image URL.
// It will create a minimal template with the image URL and arch derived from the image name
// and also set the default instance name to the image name, but stripped of generic tags.
func imageTemplate(tmpl *Template, locator string) bool {
	if !imageURLRegex.MatchString(locator) {
		return false
	}

	var imageArch limatype.Arch
	for keyword, arch := range archKeywords {
		pattern := fmt.Sprintf(`\b%s\b`, keyword)
		if regexp.MustCompile(pattern).MatchString(locator) {
			imageArch = arch
			break
		}
	}
	if imageArch == "" {
		imageArch = limatype.NewArch(runtime.GOARCH)
		logrus.Warnf("cannot determine image arch from URL %q; assuming %q", locator, imageArch)
	}
	template := `arch: %q
images:
- location: %q
  arch: %q
`
	tmpl.Bytes = fmt.Appendf(nil, template, imageArch, locator, imageArch)
	tmpl.Name = InstNameFromImageURL(locator, imageArch)
	return true
}

func InstNameFromImageURL(locator, imageArch string) string {
	// We intentionally call both path.Base and filepath.Base in case we are running on Windows.
	name := strings.ToLower(filepath.Base(path.Base(locator)))
	// Remove file format and compression file types.
	name = imageURLRegex.ReplaceAllString(name, "")
	// The Alpine "nocloud_" prefix does not fit the genericTags pattern.
	name = strings.TrimPrefix(name, "nocloud_")
	for _, tag := range genericTags {
		re := regexp.MustCompile(fmt.Sprintf(`[-_.]%s\b`, tag))
		name = re.ReplaceAllString(name, "")
	}
	// Remove imageArch as well if it is the native arch.
	if limayaml.IsNativeArch(imageArch) {
		re := regexp.MustCompile(fmt.Sprintf(`[-_.]%s\b`, imageArch))
		name = re.ReplaceAllString(name, "")
	}
	// Remove timestamps from name: 8 digit date, optionally followed by
	// a delimiter and one or more digits before a word boundary.
	name = regexp.MustCompile(`[-_.]20\d{6}([-_.]\d+)?\b`).ReplaceAllString(name, "")
	// Normalize archlinux name
	name = regexp.MustCompile(`^arch\b`).ReplaceAllString(name, "archlinux")
	// Remove redundant major version, e.g. "rocky-8-8.10" becomes "rocky-8.10".
	// Unfortunately regexp doesn't support back references, so we have to
	// check manually if both numbers are the same.
	re := regexp.MustCompile(`-(\d+)-(\d+)\.`)
	name = re.ReplaceAllStringFunc(name, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		if submatch[1] == submatch[2] {
			// Replace -X-X. with -X.
			return "-" + submatch[1] + "."
		}
		return match
	})
	return name
}

// SeemsTemplateURL returns true if the arg is a URL using the template scheme.
// When it returns true, it also returns the template name.
func SeemsTemplateURL(arg string) (isTemplate bool, templateName string) {
	u, err := url.Parse(arg)
	if err != nil {
		return false, ""
	}
	if u.Scheme == "template" {
		return true, path.Join(u.Host, u.Path)
	}
	return false, ""
}

// SeemsHTTPURL returns true if the arg is a URL using the http or https scheme.
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

// SeemsFileURL returns true if the arg is a URL using the file scheme.
func SeemsFileURL(arg string) bool {
	u, err := url.Parse(arg)
	if err != nil {
		return false
	}
	return u.Scheme == "file"
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
