/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package editflags

import (
	"fmt"
	"math/bits"
	"runtime"
	"strconv"
	"strings"

	"github.com/pbnjay/memory"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

// RegisterEdit registers flags related to in-place YAML modification, for `limactl edit`.
func RegisterEdit(cmd *cobra.Command) {
	registerEdit(cmd, "")
}

func registerEdit(cmd *cobra.Command, commentPrefix string) {
	flags := cmd.Flags()

	flags.Int("cpus", 0, commentPrefix+"number of CPUs") // Similar to colima's --cpu, but the flag name is slightly different (cpu vs cpus)
	_ = cmd.RegisterFlagCompletionFunc("cpus", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		var res []string
		for _, f := range completeCPUs(runtime.NumCPU()) {
			res = append(res, strconv.Itoa(f))
		}
		return res, cobra.ShellCompDirectiveNoFileComp
	})

	flags.IPSlice("dns", nil, commentPrefix+"specify custom DNS (disable host resolver)") // colima-compatible

	flags.Float32("memory", 0, commentPrefix+"memory in GiB") // colima-compatible
	_ = cmd.RegisterFlagCompletionFunc("memory", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		var res []string
		for _, f := range completeMemoryGiB(memory.TotalMemory()) {
			res = append(res, fmt.Sprintf("%.1f", f))
		}
		return res, cobra.ShellCompDirectiveNoFileComp
	})

	flags.StringSlice("mount", nil, commentPrefix+"directories to mount, suffix ':w' for writable (Do not specify directories that overlap with the existing mounts)") // colima-compatible

	flags.String("mount-type", "", commentPrefix+"mount type (reverse-sshfs, 9p, virtiofs)") // Similar to colima's --mount-type=(sshfs|9p|virtiofs), but "reverse-sshfs" is Lima is called "sshfs" in colima
	_ = cmd.RegisterFlagCompletionFunc("mount-type", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"reverse-sshfs", "9p", "virtiofs"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.Bool("mount-writable", false, commentPrefix+"make all mounts writable")
	flags.Bool("mount-inotify", false, commentPrefix+"enable inotify for mounts")

	flags.StringSlice("network", nil, commentPrefix+"additional networks, e.g., \"vzNAT\" or \"lima:shared\" to assign vmnet IP")
	_ = cmd.RegisterFlagCompletionFunc("network", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		// TODO: retrieve the lima:* network list from networks.yaml
		return []string{"lima:shared", "lima:bridged", "lima:host", "lima:user-v2", "vzNAT"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.Bool("rosetta", false, commentPrefix+"enable Rosetta (for vz instances)")

	flags.String("set", "", commentPrefix+"modify the template inplace, using yq syntax")

	// negative performance impact: https://gitlab.com/qemu-project/qemu/-/issues/334
	flags.Bool("video", false, commentPrefix+"enable video output (has negative performance impact for QEMU)")
}

// RegisterCreate registers flags related to in-place YAML modification, for `limactl create`.
func RegisterCreate(cmd *cobra.Command, commentPrefix string) {
	registerEdit(cmd, commentPrefix)
	flags := cmd.Flags()

	flags.String("arch", "", commentPrefix+"machine architecture (x86_64, aarch64, riscv64)") // colima-compatible
	_ = cmd.RegisterFlagCompletionFunc("arch", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"x86_64", "aarch64", "riscv64"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.String("containerd", "", commentPrefix+"containerd mode (user, system, user+system, none)")
	_ = cmd.RegisterFlagCompletionFunc("vm-type", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"user", "system", "user+system", "none"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.Float32("disk", 0, commentPrefix+"disk size in GiB") // colima-compatible
	_ = cmd.RegisterFlagCompletionFunc("memory", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"10", "30", "50", "100", "200"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.String("vm-type", "", commentPrefix+"virtual machine type (qemu, vz)") // colima-compatible
	_ = cmd.RegisterFlagCompletionFunc("vm-type", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"qemu", "vz"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.Bool("plain", false, commentPrefix+"plain mode. Disable mounts, port forwarding, containerd, etc.")
}

func defaultExprFunc(expr string) func(v *flag.Flag) (string, error) {
	return func(v *flag.Flag) (string, error) {
		return fmt.Sprintf(expr, v.Value), nil
	}
}

// YQExpressions returns YQ expressions.
func YQExpressions(flags *flag.FlagSet, newInstance bool) ([]string, error) {
	type def struct {
		flagName                 string
		exprFunc                 func(*flag.Flag) (string, error)
		onlyValidForNewInstances bool
		experimental             bool
	}
	d := defaultExprFunc
	defs := []def{
		{"cpus", d(".cpus = %s"), false, false},
		{
			"dns",
			func(_ *flag.Flag) (string, error) {
				ipSlice, err := flags.GetIPSlice("dns")
				if err != nil {
					return "", err
				}
				expr := `.dns += [`
				for i, ip := range ipSlice {
					expr += fmt.Sprintf("%q", ip)
					if i < len(ipSlice)-1 {
						expr += ","
					}
				}
				expr += `] | .dns |= unique | .hostResolver.enabled=false`
				logrus.Warnf("Disabling HostResolver, as custom DNS addresses (%v) are specified", ipSlice)
				return expr, nil
			},
			false,
			false,
		},
		{"memory", d(".memory = \"%sGiB\""), false, false},
		{
			"mount",
			func(_ *flag.Flag) (string, error) {
				ss, err := flags.GetStringSlice("mount")
				if err != nil {
					return "", err
				}
				expr := `.mounts += [`
				for i, s := range ss {
					writable := strings.HasSuffix(s, ":w")
					loc := strings.TrimSuffix(s, ":w")
					expr += fmt.Sprintf(`{"location": %q, "writable": %v}`, loc, writable)
					if i < len(ss)-1 {
						expr += ","
					}
				}
				expr += `] | .mounts |= unique_by(.location)`
				return expr, nil
			},
			false,
			false,
		},
		{"mount-type", d(".mountType = %q"), false, false},
		{"mount-inotify", d(".mountInotify = %s"), false, true},
		{"mount-writable", d(".mounts[].writable = %s"), false, false},
		{
			"network",
			func(_ *flag.Flag) (string, error) {
				ss, err := flags.GetStringSlice("network")
				if err != nil {
					return "", err
				}
				expr := `.networks += [`
				for i, s := range ss {
					// CLI syntax is still experimental (YAML syntax is out of experimental)
					switch {
					case s == "vzNAT":
						expr += `{"vzNAT": true}`
					case strings.HasPrefix(s, "lima:"):
						network := strings.TrimPrefix(s, "lima:")
						expr += fmt.Sprintf(`{"lima": %q}`, network)
					default:
						return "", fmt.Errorf("network name must be \"vzNAT\" or \"lima:*\", got %q", s)
					}
					if i < len(ss)-1 {
						expr += ","
					}
				}
				expr += `] | .networks |= unique_by(.lima)`
				return expr, nil
			},
			false,
			false,
		},
		{
			"rosetta",
			func(_ *flag.Flag) (string, error) {
				b, err := flags.GetBool("rosetta")
				if err != nil {
					return "", err
				}
				return fmt.Sprintf(".rosetta.enabled = %v | .rosetta.binfmt = %v", b, b), nil
			},
			false,
			false,
		},
		{"set", d("%s"), false, false},
		{
			"video",
			func(_ *flag.Flag) (string, error) {
				b, err := flags.GetBool("video")
				if err != nil {
					return "", err
				}
				if b {
					return ".video.display = \"default\"", nil
				}
				return ".video.display = \"none\"", nil
			},
			false,
			false,
		},
		{"arch", d(".arch = %q"), true, false},
		{
			"containerd",
			func(_ *flag.Flag) (string, error) {
				s, err := flags.GetString("containerd")
				if err != nil {
					return "", err
				}
				switch s {
				case "user":
					return `.containerd.user = true | .containerd.system = false`, nil
				case "system":
					return `.containerd.user = false | .containerd.system = true`, nil
				case "user+system", "system+user":
					return `.containerd.user = true | .containerd.system = true`, nil
				case "none":
					return `.containerd.user = false | .containerd.system = false`, nil
				default:
					return "", fmt.Errorf(`expected one of ["user", "system", "user+system", "none"], got %q`, s)
				}
			},
			true,
			false,
		},
		{"disk", d(".disk= \"%sGiB\""), true, false},
		{"vm-type", d(".vmType = %q"), true, false},
		{"plain", d(".plain = %s"), true, false},
	}
	var exprs []string
	for _, def := range defs {
		v := flags.Lookup(def.flagName)
		if v != nil && v.Changed {
			if def.experimental {
				logrus.Warnf("`--%s` is experimental", def.flagName)
			}
			if def.onlyValidForNewInstances && !newInstance {
				logrus.Warnf("`--%s` is not applicable to an existing instance (Hint: create a new instance with `limactl create --%s=%s --name=NAME`)",
					def.flagName, def.flagName, v.Value.String())
				continue
			}
			expr, err := def.exprFunc(v)
			if err != nil {
				return exprs, fmt.Errorf("error while processing flag %q: %w", def.flagName, err)
			}
			exprs = append(exprs, expr)
		}
	}
	return exprs, nil
}

func isPowerOfTwo(x int) bool {
	return bits.OnesCount(uint(x)) == 1
}

func completeCPUs(hostCPUs int) []int {
	var res []int
	for i := 1; i <= hostCPUs; i *= 2 {
		res = append(res, i)
	}
	if !isPowerOfTwo(hostCPUs) {
		res = append(res, hostCPUs)
	}
	return res
}

func completeMemoryGiB(hostMemory uint64) []float32 {
	hostMemoryHalfGiB := int(hostMemory / 2 / 1024 / 1024 / 1024)
	var res []float32
	if hostMemoryHalfGiB < 1 {
		res = append(res, 0.5)
	}
	for i := 1; i <= hostMemoryHalfGiB; i *= 2 {
		res = append(res, float32(i))
	}
	if hostMemoryHalfGiB > 1 && !isPowerOfTwo(hostMemoryHalfGiB) {
		res = append(res, float32(hostMemoryHalfGiB))
	}
	return res
}
