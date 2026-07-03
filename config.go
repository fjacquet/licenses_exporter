package main

import (
	core "github.com/fjacquet/licenses-exporter-core"
	"github.com/fjacquet/m365_licenses_exporter/internal/m365"
)

// Config is the exporter's full config: the shared core.Base (collection + otlp)
// inline, plus the vendor-specific m365 block.
type Config struct {
	core.Base `yaml:",inline"`
	M365      m365.M365Config `yaml:"m365"`
}

// loadConfig parses the file and builds the sources — the single closure body
// core.Main calls at startup and on every reload.
func loadConfig(path string) (core.Base, []core.Source, error) {
	var cfg Config
	if err := core.LoadYAML(path, &cfg); err != nil {
		return core.Base{}, nil, err
	}
	if err := cfg.Validate(); err != nil {
		return core.Base{}, nil, err
	}
	sources, err := m365.NewSources(cfg.M365)
	if err != nil {
		return core.Base{}, nil, err
	}
	return cfg.Base, sources, nil
}
