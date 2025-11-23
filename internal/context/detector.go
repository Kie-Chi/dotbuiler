package context

import (
	"bufio"
	"dotbuilder/pkg/constants"
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

	id := readOSRelease("ID")
	idLike := readOSRelease("ID_LIKE")
	name := readOSRelease("NAME")

	s := strings.ToLower(id + " " + idLike + " " + name)
	distro, base := constants.ResolveBasePMToken(s)
	info.Distro, info.BasePM = distro, base
	if info.BasePM == "unknown" && shell.CheckCommandExists("brew") {
		info.Distro = "brew"
		info.BasePM = "brew"
	}
	return info
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