package context

import (
	"bufio"
	"dotbuilder/pkg/shell"
	"os"
	"strings"
)

type SystemInfo struct {
	OS     string
	Distro string // e.g., "ubuntu", "arch", "fedora"
	BasePM string // e.g., "apt-get", "pacman"
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

	// Read os-release
	id := readOSRelease("ID")           // e.g. "ubuntu", "pop"
	idLike := readOSRelease("ID_LIKE")  // e.g. "debian", "ubuntu"
	
	// Use the actual ID as Distro for map matching
	if id != "" {
		info.Distro = id
	} else {
		info.Distro = "linux"
	}

	// Resolve PM based on combined info
	s := strings.ToLower(id + " " + idLike)
	info.BasePM = resolveBasePM(s)
	
	return info
}

func resolveBasePM(s string) string {
	switch {
	case strings.Contains(s, "ubuntu") || strings.Contains(s, "debian") || strings.Contains(s, "kali") || strings.Contains(s, "pop") || strings.Contains(s, "mint"):
		return "apt-get"
	case strings.Contains(s, "arch") || strings.Contains(s, "manjaro") || strings.Contains(s, "endeavour"):
		return "pacman"
	case strings.Contains(s, "alpine"):
		return "apk"
	case strings.Contains(s, "fedora") || strings.Contains(s, "rhel") || strings.Contains(s, "centos") || strings.Contains(s, "rocky") || strings.Contains(s, "alma"):
		// Prefer DNF on newer RHEL/CentOS
		if shell.CheckCommandExists("dnf") {
			return "dnf"
		}
		return "yum"
	case strings.Contains(s, "suse") || strings.Contains(s, "opensuse"):
		return "zypper"
	default:
		// Fallback check
		if shell.CheckCommandExists("apt-get") { return "apt-get" }
		if shell.CheckCommandExists("pacman") { return "pacman" }
		if shell.CheckCommandExists("dnf") { return "dnf" }
		if shell.CheckCommandExists("yum") { return "yum" }
		if shell.CheckCommandExists("apk") { return "apk" }
		if shell.CheckCommandExists("brew") { return "brew" }
		return "unknown"
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
			return strings.Trim(strings.Trim(line[len(key)+1:], "\""), "'")
		}
	}
	return ""
}