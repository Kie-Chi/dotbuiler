package config

import (
	"os"
    "dotbuilder/internal/context"
    "dotbuilder/pkg/constants"
	"gopkg.in/yaml.v3"
	"path/filepath"
	"fmt"
)

type Config struct {
	Include []string         	`yaml:"include"`
	Meta  Meta              	`yaml:"meta"`
	Vars  map[string]string 	`yaml:"vars"`
	Scrpits map[string]string   `yaml:"scripts"`
	Pkgs  []Package         	`yaml:"pkgs"`
	Files []File            	`yaml:"files"`
	Tasks []Task            	`yaml:"tasks"`
}

type Meta struct {
	Name string `yaml:"name"`
	Ver  string `yaml:"ver"`
}

type Package struct {
	Name    string            `yaml:"name"`
	Map     map[string]string `yaml:"map"`
	Def     string            `yaml:"def"` // Default name
	Manager string            `yaml:"manager"`
	PM      string            `yaml:"pm"` // Alias for Manager
	Ignore  bool              `yaml:"ignore"`
	Deps    []string          `yaml:"deps"`

	// Install Lifecycle
	Check string `yaml:"check"`
	Pre   string `yaml:"pre"`
	Exec  string `yaml:"exec"` // Custom install cmd
	Post  string `yaml:"post"`

	// PM Template (when acting as PM)
	PmInstallTpl string `yaml:"pmi"`
	PmCheckTpl   string `yaml:"pmc"`
	PmUpdateTpl  string `yaml:"pmu"`

	// Maintenance
	Upd   string `yaml:"upd"`
	Clean string `yaml:"clean"`
}

func mergeConfigs(base, incoming *Config) {
	if incoming.Meta.Name != "" {
		base.Meta.Name = incoming.Meta.Name
	}
	if incoming.Meta.Ver != "" {
		base.Meta.Ver = incoming.Meta.Ver
	}

	// Vars & Scripts: recursive merge
	for k, v := range incoming.Vars {
		base.Vars[k] = v
	}
	for k, v := range incoming.Scrpits {
		base.Scrpits[k] = v
	}

	// Pkgs, Files, Tasks: append
	base.Pkgs = append(base.Pkgs, incoming.Pkgs...)
	base.Files = append(base.Files, incoming.Files...)
	base.Tasks = append(base.Tasks, incoming.Tasks...)
}

// UnmarshalYAML supports polymorphic parse: "- git" or "- name: git"
func (p *Package) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		p.Name = value.Value
		p.Def = value.Value
		return nil
	}
	// Fallback to default struct unmarshal
	type plain Package
	return value.Decode((*plain)(p))
}

func (p *Package) GetManager() string {
	if p.PM != "" {
		return p.PM
	}
	return p.Manager
}

func (p *Package) ResolveName(sys *context.SystemInfo) string {
    realName := ""
    lookupKeys := constants.GetPkgLookupKeys(sys.Distro, sys.BasePM)
    for _, key := range lookupKeys {
        if val, ok := p.Map[key]; ok {
            realName = val
            break
        }
    }

    if realName == "" {
        realName = p.Def
    }

    if realName == "" {
        realName = p.Name
    }

    return realName
}

type File struct {
    ID          string      `yaml:"id"`
	Src         string 	    `yaml:"src"`
	Dest        string 	    `yaml:"dest"`
    Override    bool        `yaml:"override"`
    Check       string      `yaml:"check"`
    Append      bool        `yaml:"append"`
    OverrideIf  string      `yaml:"override_if"`
	Tpl         bool   	    `yaml:"tpl"`
	Deps        []string	`yaml:"deps"`
}

type Task struct {
	ID    string            `yaml:"id"`
	Deps  []string          `yaml:"deps"`
	Vars  map[string]string `yaml:"vars"`
	Check string            `yaml:"check"`
	On    map[string]string `yaml:"on"`
	Run   string            `yaml:"run"`
}

func loadRecursive(path string, visited map[string]bool) (*Config, error) {
	if visited[path] {
		return nil, fmt.Errorf("cyclic include detected: %s", path)
	}
	visited[path] = true
	defer delete(visited, path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var currentCfg Config
	if err := yaml.Unmarshal(data, &currentCfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if currentCfg.Vars == nil { currentCfg.Vars = make(map[string]string) }
	if currentCfg.Scrpits == nil { currentCfg.Scrpits = make(map[string]string) }
	
	if len(currentCfg.Include) == 0 {
		return &currentCfg, nil
	}
	finalConfig := &Config{
		Vars:    make(map[string]string),
		Scrpits: make(map[string]string),
	}
	baseDir := filepath.Dir(path)
	for _, includePath := range currentCfg.Include {
		absIncludePath := filepath.Join(baseDir, includePath)
		includedCfg, err := loadRecursive(absIncludePath, visited)
		if err != nil {
			return nil, err
		}
		mergeConfigs(finalConfig, includedCfg)
	}
	mergeConfigs(finalConfig, &currentCfg)
	return finalConfig, nil
}

func Load(path string) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return loadRecursive(absPath, make(map[string]bool))
}
