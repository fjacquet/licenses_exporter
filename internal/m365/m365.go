package m365

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	core "github.com/fjacquet/licenses-exporter-core"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// graphScopes requests the app-only default scope; the app registration must be
// granted Organization.Read.All (or Directory.Read.All) — see docs/deployment.
var graphScopes = []string{"https://graph.microsoft.com/.default"}

// NewSources builds one core.Source per configured tenant.
func NewSources(cfg M365Config) ([]core.Source, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	var out []core.Source
	for _, t := range cfg.Tenants {
		secret, err := core.ResolveSecret(t.ClientSecret, t.ClientSecretFile)
		if err != nil {
			return nil, fmt.Errorf("m365 tenant %q: %w", t.Instance, err)
		}
		cred, err := azidentity.NewClientSecretCredential(t.TenantID, t.ClientID, secret, nil)
		if err != nil {
			return nil, fmt.Errorf("m365 tenant %q credential: %w", t.Instance, err)
		}
		client, err := msgraphsdk.NewGraphServiceClientWithCredentials(cred, graphScopes)
		if err != nil {
			return nil, fmt.Errorf("m365 tenant %q client: %w", t.Instance, err)
		}
		out = append(out, &source{instance: t.Instance, lister: graphSkuLister{client: client}})
	}
	return out, nil
}
