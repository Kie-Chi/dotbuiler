package context

import (
	"bufio"
	"dotbuilder/pkg/shell"
	"os"
	"strings"
)

type SystemInfo struct {
	OS     string
	Distro string
	BasePM string
}

func Detect() *SystemInfo {
	info := &SystemInfo{OS: "linux"}

	// macOS detection
	if _, err := os.Stat("/Applications"); err == nil {
		if shell.CheckCommandExists("sw_vers") {
			info.OS = "darwin"
			info.Distro = "macos"
			info.BasePM = "brew"
			return info
		}
	}

	id := readOSRelease("ID")
	idLike := readOSRelease("ID_LIKE")
	name := readOSRelease("NAME")

	s := strings.ToLower(id + " " + idLike + " " + name)
	distro, base := resolveDistroAndPM(s)
	
	info.Distro = distro
	info.BasePM = base
	
	return info
}

func resolveDistroAndPM(s string) (string, string) {
	switch {
	case strings.Contains(s, "ubuntu") || strings.Contains(s, "debian") || strings.Contains(s, "kali") || strings.Contains(s, "pop"):
		return "apt", "apt-get"
	case strings.Contains(s, "arch") || strings.Contains(s, "manjaro") || strings.Contains(s, "endeavour"):
		return "pacman", "pacman"
	case strings.Contains(s, "alpine"):
		return "apk", "apk"
	case strings.Contains(s, "fedora"):
		return "dnf", "dnf"
	case strings.Contains(s, "centos") || strings.Contains(s, "rhel") || strings.Contains(s, "redhat") || strings.Contains(s, "rocky") || strings.Contains(s, "alma"):
		// Prefer DNF on newer RHEL/CentOS
		if shell.CheckCommandExists("dnf") {
			return "dnf", "dnf"
		}
		return "yum", "yum"
	case strings.Contains(s, "suse") || strings.Contains(s, "opensuse"):
		return "zypper", "zypper"
	default:
		// Fallback check
		if shell.CheckCommandExists("apt-get") { return "apt", "apt-get" }
		if shell.CheckCommandExists("pacman") { return "pacman", "pacman" }
		if shell.CheckCommandExists("dnf") { return "dnf", "dnf" }
		if shell.CheckCommandExists("brew") { return "brew", "brew" }
		return "unknown", "unknown"
	}
}

func IsRoot() bool {
	return os.Geteuid() == 0
}

func readOSRelease(key string) string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, key+"=") {
			return strings.Trim(line[len(key)+1:], "\"")
		}
	}
	return ""
}