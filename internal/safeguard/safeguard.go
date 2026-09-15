// Package safeguard blocks privileged runs so cloud models never see root’s view of the disk.
package safeguard

import (
	"errors"
	"os"
	"strings"
)

var (
	// ErrSudo is returned when the process was launched with sudo.
	ErrSudo = errors.New("easy refuses to run under sudo: searching as root would send other users' files to a cloud model")
	// ErrRoot is returned when euid is 0 without EASY_ALLOW_ROOT=1.
	ErrRoot = errors.New("easy refuses to run as root (set EASY_ALLOW_ROOT=1 only on a machine you own; this is unsupported)")
)

// Check refuses sudo and, unless EASY_ALLOW_ROOT=1, euid 0.
func Check(getenv func(string) string, euid int) error {
	if getenv == nil {
		getenv = os.Getenv
	}
	if strings.TrimSpace(getenv("SUDO_USER")) != "" || strings.TrimSpace(getenv("SUDO_UID")) != "" {
		return ErrSudo
	}
	if euid == 0 && strings.TrimSpace(getenv("EASY_ALLOW_ROOT")) != "1" {
		return ErrRoot
	}
	return nil
}
