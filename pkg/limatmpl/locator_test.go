package limatmpl

import (
	"testing"

	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
)

type digestTest struct {
	locator string
	fileRef string
	digest  string
	error   string
}

func TestDigestSuffix(t *testing.T) {
	tests := []digestTest{
		// may looks like a digest, but is (intentionally) allowed as part of a template name
		{"tmpl.sh", "tmpl.sh", "", ""},
		{"tmpl.sh@v1", "tmpl.sh@v1", "", ""},
		{"tmpl.sh@1.0", "tmpl.sh@1.0", "", ""},
		// this is an error though because it looks like a (too short) digest
		{"tmpl.sh@1", "", "", "fewer than"},

		// can always append a file extension to use a digest string as part of a template name
		{"tmpl@1.sh", "tmpl@1.sh", "", ""},
		{"template://my@1234567", "template://my", "sha256:1234567", ""},
		{"template://my@1234567.yaml", "template://my@1234567.yaml", "", ""},
		{"my@sha256:1234567", "my", "sha256:1234567", ""},
		{"my@sha256:1234567.yaml", "my@sha256:1234567.yaml", "", ""},

		// digest inside the middle of a URL is always ignored
		{"https://example.com/templates@sha256:1234567/my.yaml", "https://example.com/templates@sha256:1234567/my.yaml", "", ""},

		// locators with digests
		{"tmpl.sh@1234567", "tmpl.sh", "sha256:1234567", ""},
		{"tmpl.sh@sha256:1234567", "tmpl.sh", "sha256:1234567", ""},

		// invalid locators
		{"tmpl.sh@invalid:1234567", "", "", "unavailable digest"},
		{"tmpl.sh@abcdef", "tmpl.sh", "", "fewer than"},
	}
	for _, test := range tests {
		t.Run(test.locator, func(t *testing.T) {
			tmpl := Template{Locator: test.locator}
			err := tmpl.splitOffDigest()
			if test.error != "" {
				assert.ErrorContains(t, err, test.error, test.locator)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, tmpl.Locator, test.fileRef)
				if test.digest == "" {
					assert.Equal(t, tmpl.digest, "")
				} else {
					actual := digest.NewDigestFromEncoded(tmpl.algorithm, tmpl.digest)
					assert.Equal(t, actual.String(), test.digest)
				}
			}
		})
	}
}
