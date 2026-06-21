package localssh

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

func NullDevice() string {
	if runtime.GOOS == "windows" {
		return "NUL"
	}
	return "/dev/null"
}

func EnsureInstalled(commands ...string) error {
	for _, name := range commands {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("couldn't find the %q binary in PATH - install the OpenSSH client tools and try again", name)
		}
	}
	return nil
}

func BaseArgs(identityFile, port string) []string {
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=" + NullDevice(),
		"-o", "IdentitiesOnly=yes",
		"-o", "PreferredAuthentications=publickey",
		"-i", identityFile,
	}
	if port != "" {
		args = append(args, "-p", port)
	}
	return args
}

func SCPArgs(identityFile, port string) []string {
	return []string{
		"-i", identityFile,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=" + NullDevice(),
		"-P", port,
		"-C",
	}
}

func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func ConfigPathHint() string {
	if runtime.GOOS == "windows" {
		return `%USERPROFILE%\.ssh\config`
	}
	return "~/.ssh/config"
}
