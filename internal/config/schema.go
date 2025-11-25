package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Meta  Meta              `yaml:"meta"`
	Vars  map[string]string `yaml:"vars"`
	Pkgs  []Package         `yaml:"pkgs"`
	Files []File            `yaml:"files"`
	Tasks []Task            `yaml:"tasks"`
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

type File struct {
    ID       string     `yaml:"id"`
	Src      string 	`yaml:"src"`
	Dest     string 	`yaml:"dest"`
    Override bool       `yaml:"override"`
    Check    string     `yaml:"check"`
    Append   bool       `yaml:"append"`
	Tpl      bool   	`yaml:"tpl"`
	Deps     []string	`yaml:"deps"`
}

type Task struct {
	ID    string            `yaml:"id"`
	Deps  []string          `yaml:"deps"`
	Vars  map[string]string `yaml:"vars"`
	Check string            `yaml:"check"`
	On    map[string]string `yaml:"on"`
	Run   string            `yaml:"run"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
