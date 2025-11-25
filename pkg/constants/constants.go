package constants

import (
    "strings"
)

type PMTemplates struct {
    Check   string
    Install string
    Update  string
}

var PMLockGroups = map[string]string{
	"apt":     "dpkg",
	"apt-get": "dpkg",
	"dpkg":    "dpkg",
	"nala":    "dpkg",

	"yum":     "rpm",
	"dnf":     "rpm",
	"rpm":     "rpm",

	"pacman":  "pacman",
	"yay":     "pacman",
	"paru":    "pacman",

	"apk":     "apk",
	"snap":    "snap",
	"zypper":  "zypper",
}

var PMNeedsSudo = map[string]bool{
	"apt-get": true, "apt": true,
	"pacman":  true,
	"dnf":     true, "yum": true,
	"zypper":  true,
	"apk":     true,
	"snap":    true,
}

var SystemUpdateCmds = map[string]string{
	"apt-get": "apt-get update",
	"apt":     "apt update",
	"pacman":  "pacman -Sy",
	"apk":     "apk update",
	"brew":    "brew update",
	"dnf":     "dnf check-update",
	"yum":     "yum check-update",
	"zypper":  "zypper refresh",
}

var BaseBatchTemplates = map[string]string{
	"apt-get": "apt-get install -y {{names}}",
	"pacman":  "pacman -S --noconfirm {{names}}",
	"apk":     "apk add {{names}}",
	"dnf":     "dnf install -y {{names}}",
	"yum":     "yum install -y {{names}}",
	"zypper":  "zypper install -y {{names}}",
	"brew":    "brew install {{names}}",
    "pip":     "pip install {{names}}",
	"npm":     "npm install -g {{names}}",
	"cargo":   "cargo install {{names}}",
}

var BaseSingleTemplates = map[string]string{
	"apt-get": "apt-get install -y {{name}}",
	"pacman":  "pacman -S --noconfirm {{name}}",
	"apk":     "apk add {{name}}",
	"dnf":     "dnf install -y {{name}}",
	"yum":     "yum install -y {{name}}",
	"zypper":  "zypper install -y {{name}}",
	"brew":    "brew install {{name}}",
}

var BaseCheckTemplates = map[string]string{
    "apt-get": "dpkg -s {{name}}",
    "apt":     "dpkg -s {{name}}",
    "pacman":  "pacman -Qi {{name}}",
    "dnf":     "rpm -q {{name}}",
    "brew":    "brew list {{name}}",
}

var DefaultPMTemplatesMap = map[string]PMTemplates{
	"brew":  {Check: "brew list {{.name}}", Install: "brew install {{.name}}", Update: "brew upgrade {{.name}}"},
	"npm":   {Check: "npm ls -g {{.name}}", Install: "npm install -g {{.name}}", Update: "npm update -g {{.name}}"},
	"cargo": {Check: "cargo install --list | grep '^{{.name}}'", Install: "cargo install {{.name}}", Update: "cargo install {{.name}} --force"},
	"conda": {Check: "conda list {{.name}}", Install: "conda install -y {{.name}}", Update: "conda update -y {{.name}}"},
	"pip":   {Check: "pip show {{.name}}", Install: "pip install {{.name}}", Update: "pip install --upgrade {{.name}}"},

	"gem": {
		Check:   "gem list -i {{.name}}",
		Install: "gem install {{.name}}",
		Update:  "gem update {{.name}}",
	},
	"go": {
		Check:   "ls $(go env GOPATH)/bin/{{.name}}",
		Install: "go install {{.name}}@latest",
		Update:  "go install {{.name}}@latest",
	},
	"snap": {
		Check:   "snap list {{.name}}",
		Install: "snap install {{.name}}",
		Update:  "snap refresh {{.name}}",
	},
	"flatpak": {
		Check:   "flatpak list --app | grep {{.name}}",
		Install: "flatpak install -y {{.name}}",
		Update:  "flatpak update -y {{.name}}",
	},
}

var PMAliases = map[string][]string{
	"apt-get": {"apt"},          // DEB
	"yum":     {"dnf"},          // RHEL
	"dnf":     {"yum"},          // RHEL
	"pacman":  {"yay", "paru"},  // Arch
}

func GetPkgLookupKeys(distro, basePM string) []string {
    keys := []string{distro, basePM}

    if aliases, ok := PMAliases[basePM]; ok {
        keys = append(keys, aliases...)
    }

    return keys
}

func GetPMTemplates(pm string) (string, string, string) {
    if t, ok := DefaultPMTemplatesMap[pm]; ok {
        return t.Check, t.Install, t.Update
    }
    return "", "", ""
}

func BuildBatchInstallCmd(basePM string, names []string, isRoot bool) string {
    tpl, ok := BaseBatchTemplates[basePM]
    if !ok {
        return basePM + " install " + strings.Join(names, " ")
    }
    cmd := strings.ReplaceAll(tpl, "{{names}}", strings.Join(names, " "))
    if PMNeedsSudo[basePM] && !isRoot {
        return "sudo " + cmd
    }
    return cmd
}

func BuildSingleInstallCmd(basePM, name string, isRoot bool) string {
    tpl, ok := BaseSingleTemplates[basePM]
    if !ok {
        return basePM + " install " + name
    }
    cmd := strings.ReplaceAll(tpl, "{{name}}", name)
    if PMNeedsSudo[basePM] && !isRoot {
        return "sudo " + cmd
    }
    return cmd
}
