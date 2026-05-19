package autounattend_test

import (
	"strings"
	"testing"

	"github.com/lima-vm/lima/v2/pkg/windows/autounattend"
)

func baseConfig() autounattend.Config {
	return autounattend.Config{
		Hostname:            "lima-win",
		Username:            "limauser",
		Password:            "T3stP@ssw0rd!",
		SSHPublicKeys:       []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA user@host"},
		WindowsEditionIndex: 1,
	}
}

func TestGenerate_ContainsRequiredPasses(t *testing.T) {
	xml, err := autounattend.Generate(baseConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, pass := range []string{"windowsPE", "specialize", "oobeSystem"} {
		if !strings.Contains(string(xml), pass) {
			t.Errorf("missing pass: %s", pass)
		}
	}
}

func TestGenerate_ContainsVirtIODriverPaths(t *testing.T) {
	xml, err := autounattend.Generate(baseConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, driver := range []string{"viostor", "NetKVM", "viofs"} {
		if !strings.Contains(string(xml), driver) {
			t.Errorf("missing VirtIO driver path for: %s", driver)
		}
	}
}

func TestGenerate_SSHKeyPresent(t *testing.T) {
	xml, err := autounattend.Generate(baseConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(xml), "ssh-ed25519") {
		t.Error("SSH public key missing from output")
	}
}

func TestGenerate_PasswordNeverPlaintext(t *testing.T) {
	cfg := baseConfig()
	xml, err := autounattend.Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(xml), cfg.Password) {
		t.Error("SECURITY: plaintext password present in generated XML")
	}
	if !strings.Contains(string(xml), "<PlainText>false</PlainText>") {
		t.Error("PlainText flag not set to false")
	}
}

func TestGenerate_ValidationErrors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*autounattend.Config)
	}{
		{"empty hostname", func(c *autounattend.Config) { c.Hostname = "" }},
		{"hostname too long", func(c *autounattend.Config) { c.Hostname = "this-is-way-too-long" }},
		{"empty password", func(c *autounattend.Config) { c.Password = "" }},
		{"no ssh keys", func(c *autounattend.Config) { c.SSHPublicKeys = nil }},
		{"bad edition index", func(c *autounattend.Config) { c.WindowsEditionIndex = -1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseConfig()
			tc.mutate(&cfg)
			if _, err := autounattend.Generate(cfg); err == nil {
				t.Errorf("expected error for %q, got nil", tc.name)
			}
		})
	}
}

func TestSanitiseHostname(t *testing.T) {
	cases := []struct{ in, want string }{
		{"my-vm", "my-vm"},
		{"my_special_vm!", "my-special-vm"},
		{"this-is-way-too-long-for-netbios", "this-is-way-too"},
		{"---", "lima-windows"},
		{"", "lima-windows"},
	}
	for _, tc := range cases {
		got := autounattend.SanitiseHostname(tc.in)
		if got != tc.want {
			t.Errorf("SanitiseHostname(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
