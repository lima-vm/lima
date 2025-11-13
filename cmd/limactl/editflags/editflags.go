// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package editflags

import (
	"errors"
	"fmt"
	"math/bits"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/pbnjay/memory"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/lima-vm/lima/v2/pkg/localpathutil"
	"github.com/lima-vm/lima/v2/pkg/registry"
)

// RegisterEdit registers flags related to in-place YAML modification, for `limactl edit`.
func RegisterEdit(cmd *cobra.Command, commentPrefix string) {
	flags := cmd.Flags()

	flags.Int("cpus", 0, commentPrefix+"Number of CPUs") // Similar to colima's --cpu, but the flag name is slightly different (cpu vs cpus)
	_ = cmd.RegisterFlagCompletionFunc("cpus", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		var res []string
		for _, f := range completeCPUs(runtime.NumCPU()) {
			res = append(res, strconv.Itoa(f))
		}
		return res, cobra.ShellCompDirectiveNoFileComp
	})

	flags.IPSlice("dns", nil, commentPrefix+"Specify custom DNS (disable host resolver)") // colima-compatible

	flags.Float32("memory", 0, commentPrefix+"Memory in GiB") // colima-compatible
	_ = cmd.RegisterFlagCompletionFunc("memory", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		var res []string
		for _, f := range completeMemoryGiB(memory.TotalMemory()) {
			res = append(res, fmt.Sprintf("%.1f", f))
		}
		return res, cobra.ShellCompDirectiveNoFileComp
	})

	flags.StringSlice("mount", nil, commentPrefix+"Directories to mount, suffix ':w' for writable (Do not specify directories that overlap with the existing mounts)") // colima-compatible
	flags.StringSlice("mount-only", nil, commentPrefix+"Similar to --mount, but overrides the existing mounts")

	flags.Bool("mount-none", false, commentPrefix+"Remove all mounts")

	flags.String("mount-type", "", commentPrefix+"Mount type (reverse-sshfs, 9p, virtiofs)") // Similar to colima's --mount-type=(sshfs|9p|virtiofs), but "reverse-sshfs" is Lima is called "sshfs" in colima
	_ = cmd.RegisterFlagCompletionFunc("mount-type", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"reverse-sshfs", "9p", "virtiofs"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.Bool("mount-writable", false, commentPrefix+"Make all mounts writable")
	flags.Bool("mount-inotify", false, commentPrefix+"Enable inotify for mounts")

	flags.StringSlice("network", nil, commentPrefix+"Additional networks, e.g., \"vzNAT\" or \"lima:shared\" to assign vmnet IP")
	_ = cmd.RegisterFlagCompletionFunc("network", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		// TODO: retrieve the lima:* network list from networks.yaml
		return []string{"lima:shared", "lima:bridged", "lima:host", "lima:user-v2", "vzNAT"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.Bool("rosetta", false, commentPrefix+"Enable Rosetta (for vz instances)")

	flags.StringArray("set", []string{}, commentPrefix+"Modify the template inplace, using yq syntax. Can be passed multiple times.")

	flags.Uint16("ssh-port", 0, commentPrefix+"SSH port (0 for random)") // colima-compatible
	_ = cmd.RegisterFlagCompletionFunc("ssh-port", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		// Until Lima v2.0, 60022 was the default SSH port for the instance named "default".
		return []string{"60022"}, cobra.ShellCompDirectiveNoFileComp
	})

	// negative performance impact: https://gitlab.com/qemu-project/qemu/-/issues/334
	flags.Bool("video", false, commentPrefix+"Enable video output (has negative performance impact for QEMU)")

	flags.Float32("disk", 0, commentPrefix+"Disk size in GiB") // colima-compatible
	_ = cmd.RegisterFlagCompletionFunc("disk", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"10", "30", "50", "100", "200"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.String("vm-type", "", commentPrefix+"Virtual machine type")
	_ = cmd.RegisterFlagCompletionFunc("vm-type", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		var drivers []string
		for k := range registry.List() {
			drivers = append(drivers, k)
		}
		return drivers, cobra.ShellCompDirectiveNoFileComp
	})
}

// RegisterCreate registers flags related to in-place YAML modification, for `limactl create`.
func RegisterCreate(cmd *cobra.Command, commentPrefix string) {
	RegisterEdit(cmd, commentPrefix)
	flags := cmd.Flags()

	flags.String("arch", "", commentPrefix+"Machine architecture (x86_64, aarch64, riscv64, armv7l, s390x, ppc64le)") // colima-compatible
	_ = cmd.RegisterFlagCompletionFunc("arch", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"x86_64", "aarch64", "riscv64", "armv7l", "s390x", "ppc64le"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.String("containerd", "", commentPrefix+"containerd mode (user, system, user+system, none)")
	_ = cmd.RegisterFlagCompletionFunc("containerd", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"user", "system", "user+system", "none"}, cobra.ShellCompDirectiveNoFileComp
	})

	flags.Bool("plain", false, commentPrefix+"Plain mode. Disables mounts, port forwarding, containerd, etc.")

	flags.StringArray("port-forward", nil, commentPrefix+"Port forwards (host:guest), e.g., '8080:80' or '9090:9090,static=true' for static port-forwards")
	_ = cmd.RegisterFlagCompletionFunc("port-forward", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"8080:80", "3000:3000", "8080:80,static=true"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func defaultExprFunc(expr string) func(v *flag.Flag) ([]string, error) {
	return func(v *flag.Flag) ([]string, error) {
		return []string{fmt.Sprintf(expr, v.Value)}, nil
	}
}

func ParsePortForward(spec string) (hostPort, guestPort string, isStatic bool, err error) {
	parts := strings.Split(spec, ",")
	if len(parts) > 2 {
		return "", "", false, fmt.Errorf("invalid port forward format %q, expected HOST:GUEST or HOST:GUEST,static=true", spec)
	}

	portParts := strings.Split(strings.TrimSpace(parts[0]), ":")
	if len(portParts) != 2 {
		return "", "", false, fmt.Errorf("invalid port forward format %q, expected HOST:GUEST", parts[0])
	}

	hostPort = strings.TrimSpace(portParts[0])
	guestPort = strings.TrimSpace(portParts[1])

	if len(parts) == 2 {
		staticPart := strings.TrimSpace(parts[1])
		if staticValue, ok := strings.CutPrefix(staticPart, "static="); ok {
			isStatic, err = strconv.ParseBool(staticValue)
			if err != nil {
				return "", "", false, fmt.Errorf("invalid value for static parameter: %q", staticValue)
			}
		} else {
			return "", "", false, fmt.Errorf("invalid parameter %q, expected 'static=' followed by a boolean value", staticPart)
		}
	}

	return hostPort, guestPort, isStatic, nil
}

func BuildPortForwardExpression(portForwards []string) (string, error) {
	if len(portForwards) == 0 {
		return "", nil
	}

	expr := `.portForwards += [`
	for i, spec := range portForwards {
		hostPort, guestPort, isStatic, err := ParsePortForward(spec)
		if err != nil {
			return "", err
		}
		expr += fmt.Sprintf(`{"guestPort": %q, "hostPort": %q, "static": %v}`, guestPort, hostPort, isStatic)
		if i < len(portForwards)-1 {
			expr += ","
		}
	}
	expr += `]`
	return expr, nil
}

func buildMountListExpression(ss []string) (string, error) {
	expr := `[`
	for i, s := range ss {
		writable := strings.HasSuffix(s, ":w")
		loc := strings.TrimSuffix(s, ":w")
		loc, err := localpathutil.Expand(loc)
		if err != nil {
			return "", err
		}
		expr += fmt.Sprintf(`{"location": %q, "mountPoint": %q, "writable": %v}`, loc, loc, writable)
		if i < len(ss)-1 {
			expr += ","
		}
	}
	expr += `]`
	return expr, nil
}

// YQExpressions returns YQ expressions.
func YQExpressions(flags *flag.FlagSet, newInstance bool) ([]string, error) {
	type def struct {
		flagName                 string
		exprFunc                 func(*flag.Flag) ([]string, error)
		onlyValidForNewInstances bool
		experimental             bool
	}
	d := defaultExprFunc
	defs := []def{
		{"cpus", d(".cpus = %s"), false, false},
		{
			"dns",
			func(_ *flag.Flag) ([]string, error) {
				ipSlice, err := flags.GetIPSlice("dns")
				if err != nil {
					return nil, err
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
				return []string{expr}, nil
			},
			false,
			false,
		},
		{"memory", d(".memory = \"%sGiB\""), false, false},
		{
			"mount",
			func(_ *flag.Flag) ([]string, error) {
				ss, err := flags.GetStringSlice("mount")
				slices.Reverse(ss)
				if err != nil {
					return nil, err
				}
				mountListExpr, err := buildMountListExpression(ss)
				if err != nil {
					return nil, err
				}
				// mount options take precedence over template settings
				expr := fmt.Sprintf(".mounts = %s + .mounts", mountListExpr)
				mountOnly, err := flags.GetStringSlice("mount-only")
				if err != nil {
					return nil, err
				}
				if len(mountOnly) > 0 {
					return nil, errors.New("flag `--mount` conflicts with `--mount-only`")
				}
				return []string{expr}, nil
			},
			false,
			false,
		},
		{
			"mount-only",
			func(_ *flag.Flag) ([]string, error) {
				ss, err := flags.GetStringSlice("mount-only")
				if err != nil {
					return nil, err
				}
				mountListExpr, err := buildMountListExpression(ss)
				if err != nil {
					return nil, err
				}
				expr := `.mounts = ` + mountListExpr
				return []string{expr}, nil
			},
			false,
			false,
		},
		{
			"mount-none",
			func(_ *flag.Flag) ([]string, error) {
				incompatibleFlagNames := []string{"mount", "mount-only"}
				for _, name := range incompatibleFlagNames {
					ss, err := flags.GetStringSlice(name)
					if err != nil {
						return nil, err
					}
					if len(ss) > 0 {
						return nil, errors.New("flag `--mount-none` conflicts with `" + name + "`")
					}
				}
				return []string{".mounts = null"}, nil
			},
			false,
			false,
		},
		{"mount-type", d(".mountType = %q"), false, false},
		{"vm-type", d(".vmType = %q"), false, false},
		{"mount-inotify", d(".mountInotify = %s"), false, true},
		{"mount-writable", d(".mounts[].writable = %s"), false, false},
		{
			"network",
			func(_ *flag.Flag) ([]string, error) {
				ss, err := flags.GetStringSlice("network")
				if err != nil {
					return nil, err
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
						return nil, fmt.Errorf("network name must be \"vzNAT\" or \"lima:*\", got %q", s)
					}
					if i < len(ss)-1 {
						expr += ","
					}
				}
				expr += `] | .networks |= unique_by(.lima)`
				return []string{expr}, nil
			},
			false,
			false,
		},

		{
			"rosetta",
			func(_ *flag.Flag) ([]string, error) {
				b, err := flags.GetBool("rosetta")
				if err != nil {
					return nil, err
				}
				return []string{fmt.Sprintf(".vmOpts.vz.rosetta.enabled = %v | .vmOpts.vz.rosetta.binfmt = %v", b, b)}, nil
			},
			false,
			false,
		},
		{"set", func(v *flag.Flag) ([]string, error) {
			return v.Value.(flag.SliceValue).GetSlice(), nil
		}, false, false},
		{
			"video",
			func(_ *flag.Flag) ([]string, error) {
				b, err := flags.GetBool("video")
				if err != nil {
					return nil, err
				}
				if b {
					return []string{".video.display = \"default\""}, nil
				}
				return []string{".video.display = \"none\""}, nil
			},
			false,
			false,
		},
		{"ssh-port", d(".ssh.localPort = %s"), false, false},
		{"arch", d(".arch = %q"), true, false},
		{
			"containerd",
			func(_ *flag.Flag) ([]string, error) {
				s, err := flags.GetString("containerd")
				if err != nil {
					return nil, err
				}
				switch s {
				case "user":
					return []string{`.containerd.user = true | .containerd.system = false`}, nil
				case "system":
					return []string{`.containerd.user = false | .containerd.system = true`}, nil
				case "user+system", "system+user":
					return []string{`.containerd.user = true | .containerd.system = true`}, nil
				case "none":
					return []string{`.containerd.user = false | .containerd.system = false`}, nil
				default:
					return nil, fmt.Errorf(`expected one of ["user", "system", "user+system", "none"], got %q`, s)
				}
			},
			true,
			false,
		},
		{"disk", d(".disk= \"%sGiB\""), false, false},
		{"plain", d(".plain = %s"), true, false},
		{
			"port-forward",
			func(_ *flag.Flag) ([]string, error) {
				ss, err := flags.GetStringArray("port-forward")
				if err != nil {
					return nil, err
				}
				value, err := BuildPortForwardExpression(ss)
				if err != nil {
					return nil, err
				}
				return []string{value}, nil
			},
			false,
			false,
		},
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
			newExprs, err := def.exprFunc(v)
			if err != nil {
				return exprs, fmt.Errorf("error while processing flag %q: %w", def.flagName, err)
			}
			exprs = append(exprs, newExprs...)
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
