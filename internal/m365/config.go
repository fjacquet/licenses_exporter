package m365

// M365Config is the Microsoft 365 block of the exporter config. Enabled=false
// (or an empty Tenants list) yields zero sources — the exporter then serves only
// license_build_info.
type M365Config struct {
	Enabled bool           `yaml:"enabled"`
	Tenants []TenantConfig `yaml:"tenants"`
}

// TenantConfig is one Entra tenant / Graph app registration. ClientSecret is an
// inline ${ENV} ref; ClientSecretFile is a path read at load. Exactly one is used
// (ResolveSecret governs precedence).
type TenantConfig struct {
	Instance         string `yaml:"instance"`
	TenantID         string `yaml:"tenantId"`
	ClientID         string `yaml:"clientId"`
	ClientSecret     string `yaml:"clientSecret"`
	ClientSecretFile string `yaml:"clientSecretFile"`
}
