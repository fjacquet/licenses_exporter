package main

import (
	"os"
	"path/filepath"
	"testing"

	core "github.com/fjacquet/licenses-exporter-core"
)

// TestLoadConfigParsesBaseAndM365 proves the consumer Config wires core.Base
// (collection/otlp) AND the vendor m365 block from one YAML file.
func TestLoadConfigParsesBaseAndM365(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
collection:
  interval: 3h
otlp:
  endpoint: "otel:4317"
  insecure: true
m365:
  enabled: true
  tenants:
    - instance: tenant-a
      tenantId: t-id
      clientId: c-id
      clientSecret: shhh
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := core.LoadYAML(path, &cfg); err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if cfg.Collection.Interval.Hours() != 3 {
		t.Errorf("interval = %v, want 3h", cfg.Collection.Interval)
	}
	if cfg.OTLP.Endpoint != "otel:4317" {
		t.Errorf("otlp endpoint = %q, want otel:4317", cfg.OTLP.Endpoint)
	}
	if !cfg.M365.Enabled || len(cfg.M365.Tenants) != 1 || cfg.M365.Tenants[0].Instance != "tenant-a" {
		t.Errorf("m365 block not parsed: %+v", cfg.M365)
	}
}

// TestLoadReturnsSourcesForEnabledTenant proves the App.Load closure builds a
// core.Source per enabled tenant (the wiring core.Main will drive).
func TestLoadReturnsSourcesForEnabledTenant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
collection:
  interval: 2h
m365:
  enabled: true
  tenants:
    - instance: tenant-a
      tenantId: t-id
      clientId: c-id
      clientSecret: shhh
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	base, sources, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if base.Collection.Interval.Hours() != 2 {
		t.Errorf("interval = %v, want 2h", base.Collection.Interval)
	}
	if len(sources) != 1 {
		t.Fatalf("got %d sources, want 1", len(sources))
	}
	if sources[0].Vendor() != "microsoft" || sources[0].Instance() != "tenant-a" {
		t.Errorf("source identity = %s/%s, want microsoft/tenant-a", sources[0].Vendor(), sources[0].Instance())
	}
}
