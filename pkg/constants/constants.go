package constants

import (
    "strings"
)

type PMTemplates struct {
    Check   string
    Install string
    Update  string
}

var PMNeedsSudo = map[string]bool{
    "apt-get": true,
    "pacman":  true,
}

var SystemUpdateCmds = map[string]string{
    "apt-get": "apt-get update",
    "apt":     "apt update",
    "pacman":  "pacman -Sy",
    "apk":     "apk update",
    "brew":    "brew update",
    "dnf":     "dnf check-update",
    "yum":     "yum check-update",
}

var BaseBatchTemplates = map[string]string{
    "apt-get": "apt-get install -y {{names}}",
    "pacman":  "pacman -S --noconfirm {{names}}",
    "apk":     "apk add {{names}}",
}

var BaseSingleTemplates = map[string]string{
    "apt-get": "apt-get install -y {{name}}",
    "pacman":  "pacman -S --noconfirm {{name}}",
    "apk":     "apk add {{name}}",
}

var DefaultPMTemplatesMap = map[string]PMTemplates{
    "brew":  {Check: "brew list {{.name}}", Install: "brew install {{.name}}", Update: "brew upgrade {{.name}}"},
    "npm":   {Check: "npm ls -g {{.name}}", Install: "npm install -g {{.name}}", Update: "npm update -g {{.name}}"},
    "cargo": {Check: "cargo install --list | grep '^{{.name}}'", Install: "cargo install {{.name}}", Update: "cargo install {{.name}} --force"},
    "conda": {Check: "conda list {{.name}}", Install: "conda install -y {{.name}}", Update: "conda update -y {{.name}}"},
    "pip":   {Check: "pip show {{.name}}", Install: "pip install {{.name}}", Update: "pip install --upgrade {{.name}}"},
}

func GetPMTemplates(pm string) (string, string, string) {
    if t, ok := DefaultPMTemplatesMap[pm]; ok {
        return t.Check, t.Install, t.Update
    }
    return "", "", ""
}

func ResolveBasePMToken(s string) (string, string) {
    x := strings.ToLower(s)
    switch {
    case strings.Contains(x, "ubuntu") || strings.Contains(x, "debian"):
        return "apt", "apt-get"
    case strings.Contains(x, "arch"):
        return "pacman", "pacman"
    case strings.Contains(x, "alpine"):
        return "apk", "apk"
    case strings.Contains(x, "brew"):
        return "brew", "brew"
    default:
        return "unknown", "unknown"
    }
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

