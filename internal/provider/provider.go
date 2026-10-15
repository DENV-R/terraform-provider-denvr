package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// denvrProvider implémente provider.Provider.
type denvrProvider struct {
	version string
}

// New retourne la fabrique de provider attendue par providerserver.Serve.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &denvrProvider{version: version}
	}
}

type providerModel struct {
	URL   types.String `tfsdk:"url"`
	Token types.String `tfsdk:"token"`
}

func (p *denvrProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "denvr"
	resp.Version = p.version
}

func (p *denvrProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provider DENV-R : décrivez en code votre stockage souverain et votre socle de calcul (espaces, clés, clouds connectés, règles, réseau privé, cluster Kubernetes, machines, ports).",
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "URL de la console DENV-R (ex. `https://console.denv-r.com`). À défaut, variable d'environnement `DENVR_API_URL`.",
			},
			"token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Jeton d'accès (Bearer) de portée `read-write`. À défaut, variable d'environnement `DENVR_TOKEN`.",
			},
		},
	}
}

func (p *denvrProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Priorité : valeur HCL explicite, sinon variable d'environnement.
	url := os.Getenv("DENVR_API_URL")
	if !cfg.URL.IsNull() && cfg.URL.ValueString() != "" {
		url = cfg.URL.ValueString()
	}
	token := os.Getenv("DENVR_TOKEN")
	if !cfg.Token.IsNull() && cfg.Token.ValueString() != "" {
		token = cfg.Token.ValueString()
	}

	if url == "" {
		resp.Diagnostics.AddError("URL manquante",
			"Renseignez l'attribut `url` du provider ou la variable d'environnement DENVR_API_URL.")
	}
	if token == "" {
		resp.Diagnostics.AddError("Jeton manquant",
			"Renseignez l'attribut `token` du provider ou la variable d'environnement DENVR_TOKEN.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	client := NewClient(url, token)
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *denvrProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewBucketResource,
		NewCredentialResource,
		NewCloudResource,
		NewPolicyResource,
		// Le socle mutualisé DENV-R (14/10/2026) : espace, machines nues, ports publiés.
		NewEspaceResource,
		NewMachineResource,
		NewRedirectionsResource,
	}
}

func (p *denvrProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
