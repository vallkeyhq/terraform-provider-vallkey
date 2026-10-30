package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const defaultAPIURL = "https://api.vallkey.com"

// VallkeyProvider manages ghost keys and their rules in a Vallkey account.
type VallkeyProvider struct {
	version string
}

type providerModel struct {
	APIURL types.String `tfsdk:"api_url"`
	Token  types.String `tfsdk:"token"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &VallkeyProvider{version: version} }
}

func (p *VallkeyProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "vallkey"
	resp.Version = p.version
}

func (p *VallkeyProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Vallkey ghost keys and their rules as code.",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Optional:    true,
				Description: "The API address. Defaults to VALLKEY_API_URL, then https://api.vallkey.com (an account in the EU region uses its own address, shown under Settings, API tokens).",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "An API token from Settings, API tokens. Defaults to VALLKEY_TOKEN.",
			},
		},
	}
}

func (p *VallkeyProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiURL := os.Getenv("VALLKEY_API_URL")
	if !cfg.APIURL.IsNull() && !cfg.APIURL.IsUnknown() {
		apiURL = cfg.APIURL.ValueString()
	}
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	if err := validateAPIURL(apiURL); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("api_url"), "Invalid API origin", err.Error())
		return
	}
	token := os.Getenv("VALLKEY_TOKEN")
	if !cfg.Token.IsNull() && !cfg.Token.IsUnknown() {
		token = cfg.Token.ValueString()
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(path.Root("token"), "Missing API token", "Set token in the provider block, or the VALLKEY_TOKEN environment variable, to an API token from Settings, API tokens.")
		return
	}
	client := NewClient(apiURL, token, p.version)
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *VallkeyProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewKeyResource}
}

func (p *VallkeyProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewKeyDataSource}
}
