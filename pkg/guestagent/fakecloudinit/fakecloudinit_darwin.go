// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package fakecloudinit is a fake cloud-init implementation for macOS.
//
// TODO: support other OS that does not have cloud-init.
package fakecloudinit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/sethvargo/go-password/password"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/cidata/cloudinittypes"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

func Run(ctx context.Context) error {
	const mnt = "/Volumes/cidata"
	var errs []error
	if err := enableSSHD(ctx); err != nil {
		errs = append(errs, fmt.Errorf("failed to enable SSHD: %w", err))
	}
	if err := processMetaData(ctx, mnt); err != nil {
		errs = append(errs, fmt.Errorf("failed to process meta data: %w", err))
	}
	if err := processUserData(ctx, mnt); err != nil {
		errs = append(errs, fmt.Errorf("failed to process user data: %w", err))
	}
	if err := runBootScripts(ctx); err != nil {
		errs = append(errs, fmt.Errorf("failed to run boot scripts: %w", err))
	}
	return errors.Join(errs...)
}

func enableSSHD(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "/bin/launchctl", "load", "-w", "/System/Library/LaunchDaemons/ssh.plist")
	logrus.Infof("Executing command: %v", cmd.Args)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to execute command %v: %w (output=%q)", cmd.Args, err, output)
	}
	return nil
}

func processMetaData(ctx context.Context, mnt string) error {
	metaDataPath := filepath.Join(mnt, "meta-data")
	metaDataB, err := os.ReadFile(metaDataPath)
	if err != nil {
		return fmt.Errorf("failed to read meta data file %q: %w", metaDataPath, err)
	}
	var metaData cloudinittypes.MetaData
	if err = yaml.Unmarshal(metaDataB, &metaData); err != nil {
		return fmt.Errorf("failed to unmarshal meta data YAML: %w", err)
	}

	var errs []error
	logrus.Infof("Instance ID: %q", metaData.InstanceID)
	if metaData.LocalHostname != "" {
		if err = setLocalHostname(ctx, metaData.LocalHostname); err != nil {
			errs = append(errs, fmt.Errorf("failed to set local hostname: %w", err))
		}
	}
	return errors.Join(errs...)
}

func setLocalHostname(ctx context.Context, hostname string) error {
	cmd := exec.CommandContext(ctx, "scutil", "--set", "LocalHostName", hostname)
	logrus.Infof("Executing command: %v", cmd.Args)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to execute command %v: %w (output=%q)", cmd.Args, err, output)
	}
	return nil
}

func processUserData(ctx context.Context, mnt string) error {
	userDataPath := filepath.Join(mnt, "user-data")
	userDataB, err := os.ReadFile(userDataPath)
	if err != nil {
		return fmt.Errorf("failed to read user data file %q: %w", userDataPath, err)
	}
	var userData cloudinittypes.UserData
	if err = yaml.Unmarshal(userDataB, &userData); err != nil {
		return fmt.Errorf("failed to unmarshal user data YAML: %w", err)
	}

	var errs []error
	if userData.Growpart != nil {
		logrus.Warn("growpart is not implemented")
	}
	if userData.PackageUpdate {
		logrus.Warn("package_update is not implemented")
	}
	if userData.PackageUpgrade {
		logrus.Warn("package_upgrade is not implemented")
	}
	if userData.PackageRebootIfRequired {
		logrus.Warn("package_reboot_if_required is not implemented")
	}
	if len(userData.Mounts) > 0 {
		logrus.Warn("mounts is not implemented")
	}
	if userData.Timezone != "" {
		if err = setTimezone(ctx, userData.Timezone); err != nil {
			errs = append(errs, fmt.Errorf("failed to set timezone: %w", err))
		}
	}
	for _, u := range userData.Users {
		if err := createUser(ctx, &u); err != nil {
			errs = append(errs, fmt.Errorf("failed to create user %q: %w", u.Name, err))
		}
	}
	for _, entry := range userData.WriteFiles {
		if err := writeFiles(ctx, entry); err != nil {
			errs = append(errs, fmt.Errorf("failed to write file for path %q: %w", entry.Path, err))
		}
	}
	if userData.ManageResolvConf && userData.ResolvConf != nil {
		if err = setResolvConf(ctx, userData.ResolvConf); err != nil {
			errs = append(errs, fmt.Errorf("failed to apply DNS configuration: %w", err))
		}
	}
	if userData.CACerts != nil {
		logrus.Warn("ca_certs is not implemented")
	}
	if len(userData.BootCmd) > 0 {
		logrus.Warn("bootcmd is not implemented")
	}
	return errors.Join(errs...)
}

func setTimezone(ctx context.Context, timezone string) error {
	cmd := exec.CommandContext(ctx, "systemsetup", "-settimezone", timezone)
	logrus.Infof("Executing command: %v", cmd.Args)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to execute command %v: %w (output=%q)", cmd.Args, err, output)
	}
	return nil
}

func generatePassword() (string, error) {
	const pwLen = 16
	// Avoid special characters to minimize potential keyboard layout issue in GUI
	pw, err := password.Generate(pwLen, pwLen/4, 0, false, false)
	if err != nil {
		return "", fmt.Errorf("failed to generate password: %w", err)
	}
	return pw, nil
}

func populateHomeDir(ctx context.Context, uid int, homedir string) error {
	cmds := [][]string{
		{"ditto", "--noqtn", "/System/Library/User Template/English.lproj", homedir},
		{"chown", "-R", fmt.Sprintf("%d:staff", uid), homedir},
		{"chmod", "700", homedir},
	}
	for _, args := range cmds {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		logrus.Infof("Executing command: %v", cmd.Args)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to execute command %v: %w (output=%q)", cmd.Args, err, output)
		}
	}
	return nil
}

func createUser(ctx context.Context, u *cloudinittypes.User) error {
	homedir := u.Homedir
	if homedir == "" {
		return fmt.Errorf("homedir is required for user %q", u.Name)
	}
	if osutil.FileExists(homedir) {
		logrus.Debugf("homedir %q already exists, skipping user creation for user %q", homedir, u.Name)
		return nil
	}
	if u.UID == "" {
		return fmt.Errorf("uid is required for user %q", u.Name)
	}
	uid, err := strconv.Atoi(u.UID)
	if err != nil {
		return fmt.Errorf("invalid uid %q for user %q: %w", u.UID, u.Name, err)
	}
	pw, err := generatePassword()
	if err != nil {
		return fmt.Errorf("failed to generate password for user %q: %w", u.Name, err)
	}
	args := []string{
		"-addUser", u.Name,
		"-UID", u.UID,
		"-password", "-",
		"-home", homedir,
		"-admin",
	}
	if u.Gecos != "" {
		args = append(args, "-fullName", u.Gecos)
	}
	if u.Shell != "" {
		args = append(args, "-shell", u.Shell)
	}
	if u.LockPasswd != "" {
		logrus.Warn("lock_passwd field is not implemented")
	}
	cmd := exec.CommandContext(ctx, "/usr/sbin/sysadminctl", args...)
	cmd.Stdin = strings.NewReader(pw)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logrus.Infof("Executing command: %v", cmd.Args)
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute command %v: %w", cmd.Args, err)
	}

	// sysadminctl does not create the custom home directory
	if err = populateHomeDir(ctx, uid, homedir); err != nil {
		return fmt.Errorf("failed to populate home directory for user %q: %w", u.Name, err)
	}

	cmd = exec.CommandContext(ctx, "chmod", "700", homedir)
	logrus.Infof("Executing command: %v", cmd.Args)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to execute command %v: %w (output=%q)", cmd.Args, err, output)
	}

	pwPath := filepath.Join(homedir, "password")
	if err = os.WriteFile(pwPath, []byte(pw+"\n"), 0o400); err != nil {
		return fmt.Errorf("failed to write password file for user %q: %w", u.Name, err)
	}
	logrus.Infof("Created user %q. The password is stored in %q", u.Name, pwPath)

	dotSSHPath := filepath.Join(homedir, ".ssh")
	if err = os.MkdirAll(dotSSHPath, 0o700); err != nil {
		return fmt.Errorf("failed to create .ssh directory for user %q: %w", u.Name, err)
	}
	authKeysPath := filepath.Join(dotSSHPath, "authorized_keys")
	authKeysContent := strings.Join(u.SSHAuthorizedKeys, "\n")
	if err = os.WriteFile(authKeysPath, []byte(authKeysContent), 0o600); err != nil {
		return fmt.Errorf("failed to write authorized_keys file for user %q: %w", u.Name, err)
	}
	for _, f := range []string{pwPath, dotSSHPath, authKeysPath} {
		if err = os.Chown(f, uid, -1); err != nil {
			return fmt.Errorf("failed to chown %q for user %q: %w", f, u.Name, err)
		}
	}
	if u.Sudo != "" {
		if err := writeSudoers(u.Name, u.Sudo); err != nil {
			return fmt.Errorf("failed to write sudoers file for user %q: %w", u.Name, err)
		}
	}
	return nil
}

// writeSudoers appends a sudoers entry for the given user.
// writeSudoers is expected be called only once on creating the user account.
func writeSudoers(userName, sudo string) error {
	if strings.Contains(sudo, "\n") {
		return errors.New("sudo field must not contain newline characters")
	}
	if err := os.MkdirAll("/etc/sudoers.d", 0o700); err != nil {
		return fmt.Errorf("failed to create /etc/sudoers.d directory: %w", err)
	}
	sudoersPath := "/etc/sudoers.d/90-cloud-init-users"
	f, err := os.OpenFile(sudoersPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o400)
	if err != nil {
		return fmt.Errorf("failed to open sudoers file %q: %w", sudoersPath, err)
	}
	if _, err = fmt.Fprintf(f, "%s %s\n", userName, sudo); err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to write to sudoers file %q for user %q: %w", sudoersPath, userName, err)
	}
	return f.Close()
}

func writeFiles(ctx context.Context, entry cloudinittypes.WriteFile) error {
	if entry.Path == "" {
		return errors.New("path is required for write_files entry")
	}
	perm := os.FileMode(0o644)
	if entry.Permissions != "" {
		p, err := strconv.ParseUint(entry.Permissions, 8, 32)
		if err != nil {
			return fmt.Errorf("invalid permissions %q for path %q: %w", entry.Permissions, entry.Path, err)
		}
		perm = os.FileMode(p)
	}
	if err := os.MkdirAll(filepath.Dir(entry.Path), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory for path %q: %w", entry.Path, err)
	}
	if err := os.WriteFile(entry.Path, []byte(entry.Content), perm); err != nil {
		return fmt.Errorf("failed to write file for path %q: %w", entry.Path, err)
	}
	if entry.Owner != "" {
		cmd := exec.CommandContext(ctx, "chown", entry.Owner, entry.Path)
		logrus.Infof("Executing command: %v", cmd.Args)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to execute command %v: %w (output=%q)", cmd.Args, err, output)
		}
	}
	return nil
}

func setResolvConf(ctx context.Context, resolvConf *cloudinittypes.ResolvConf) error {
	// FIXME: avoid hardcoding the primary network name
	const primaryNetwork = "Ethernet"
	cmd := exec.CommandContext(ctx, "networksetup", append([]string{"-setdnsservers", primaryNetwork}, resolvConf.Nameservers...)...)
	logrus.Infof("Executing command: %v", cmd.Args)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to execute command %v: %w (output=%q)", cmd.Args, err, output)
	}
	return nil
}

func runBootScripts(ctx context.Context) error {
	dirs := []string{
		"/var/lib/cloud/scripts/per-boot",
	}
	var errs []error
	for _, dir := range dirs {
		dirEntries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("failed to read boot scripts directory %q: %w", dir, err)
		}
		for _, entry := range dirEntries {
			if entry.IsDir() {
				continue
			}
			scriptPath := filepath.Join(dir, entry.Name())
			// entry.Type().Mode() does not seem to contain permission bits
			entryInfo, err := entry.Info()
			if err != nil {
				logrus.Warnf("Skipping boot script %q due to stat error: %v", scriptPath, err)
				continue
			}
			if entryInfo.Mode().Perm()&0o111 == 0 {
				logrus.Warnf("Skipping non-executable boot script %q (%v)", scriptPath, entryInfo.Mode().Perm())
				continue
			}
			cmd := exec.CommandContext(ctx, scriptPath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			logrus.Infof("Executing command: %v", cmd.Args)
			if err := cmd.Run(); err != nil {
				errs = append(errs, fmt.Errorf("failed to execute command %v: %w", cmd.Args, err))
			}
		}
	}
	return errors.Join(errs...)
}
