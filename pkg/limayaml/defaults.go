package limayaml

import "runtime"

func FillDefault(y *LimaYAML) {
	y.Arch = resolveArch(y.Arch)
	for i := range y.Images {
		img := &y.Images[i]
		if img.Arch == "" {
			img.Arch = y.Arch
		}
	}
	if y.CPUs == 0 {
		y.CPUs = 4
	}
	if y.Memory == "" {
		y.Memory = "4GiB"
	}
	if y.Disk == "" {
		y.Disk = "100GiB"
	}
}

func resolveArch(s string) Arch {
	if s == "" || s == "default" {
		if runtime.GOOS == "amd64" {
			return AARCH64
		} else {
			return X8664
		}
	}
	return s
}
