package autounattend

import (
	_ "embed"
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"strings"
)

//go:embed template.xml
var rawTemplate string

var tmpl = template.Must(
	template.New("autounattend").
		Funcs(template.FuncMap{
			"encodePassword": encodeUTF16LE,
			"joinLines":      func(ss []string) string { return strings.Join(ss, "\n") },
			"inc":            func(n int) int { return n + 1 },
		}).
		Parse(rawTemplate),
)

type Config struct {
	Hostname            string
	Username            string
	Password            string
	SSHPublicKeys       []string
	Locale              string
	TimeZone            string
	WindowsEditionIndex int
	ExtraCommands       []string
}

func (c *Config) setDefaults() {
	if c.Username == "" {
		c.Username = "limauser"
	}
	if c.Locale == "" {
		c.Locale = "en-US"
	}
	if c.TimeZone == "" {
		c.TimeZone = "GMT Standard Time"
	}
	if c.WindowsEditionIndex == 0 {
		c.WindowsEditionIndex = 1
	}
}

func (c *Config) validate() error {
	switch {
	case c.Hostname == "":
		return fmt.Errorf("autounattend: Hostname is required")
	case len(c.Hostname) > 15:
		return fmt.Errorf("autounattend: Hostname %q exceeds 15-char NetBIOS limit", c.Hostname)
	case c.Password == "":
		return fmt.Errorf("autounattend: Password is required")
	case len(c.SSHPublicKeys) == 0:
		return fmt.Errorf("autounattend: at least one SSH public key is required")
	case c.WindowsEditionIndex < 1:
		return fmt.Errorf("autounattend: WindowsEditionIndex must be >= 1")
	}
	return nil
}

func Generate(cfg Config) ([]byte, error) {
	cfg.setDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("autounattend: render failed: %w", err)
	}
	return buf.Bytes(), nil
}

// encodeUTF16LE implements the Microsoft answer-file password encoding:
// base64( UTF-16LE( plaintext + "Password" ) )
func encodeUTF16LE(plaintext string) string {
	s := plaintext + "Password"
	b := make([]byte, len(s)*2)
	for i, r := range s {
		b[i*2] = byte(r)
		b[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// SanitiseHostname converts an arbitrary string into a valid Windows
// NetBIOS computer name (<=15 chars, alphanumeric + hyphen, no leading/trailing hyphen).
func SanitiseHostname(name string) string {
	out := make([]byte, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, byte(r))
		default:
			out = append(out, '-')
		}
	}
	s := strings.Trim(string(out), "-")
	if len(s) > 15 {
		s = s[:15]
	}
	if s == "" {
		return "lima-windows"
	}
	return s
}
