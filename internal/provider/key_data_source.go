package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = &keyDataSource{}

func NewKeyDataSource() datasource.DataSource { return &keyDataSource{} }

type keyDataSource struct {
	client *Client
}

type keyDataModel struct {
	ID          types.String `tfsdk:"id"`
	Service     types.String `tfsdk:"service"`
	Label       types.String `tfsdk:"label"`
	Project     types.String `tfsdk:"project"`
	Environment types.String `tfsdk:"environment"`
	Rules       types.String `tfsdk:"rules"`
}

func (d *keyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_key"
}

func (d *keyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a ghost key that already exists, such as one made in the dashboard.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Required: true, Description: "The key's id."},
			"service":     schema.StringAttribute{Computed: true, Description: "The provider."},
			"label":       schema.StringAttribute{Computed: true, Description: "The key's name."},
			"project":     schema.StringAttribute{Computed: true, Description: "The project, if any."},
			"environment": schema.StringAttribute{Computed: true, Description: "The environment, if any."},
			"rules":       schema.StringAttribute{Computed: true, Description: "Every rule, as JSON: the looser rules waiting to apply, or the current ones."},
		},
	}
}

func (d *keyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *Client, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *keyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data keyDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key, err := d.client.GetKey(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Could not read the key", err.Error())
		return
	}
	entry, err := d.client.KeyEntry(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Could not read the key's project", err.Error())
		return
	}
	data.Service = types.StringValue(key.Provider)
	data.Label = types.StringValue(key.Label)
	data.Project = nullableString(entry.Project)
	data.Environment = nullableString(entry.Environment)
	data.Rules = types.StringValue(CompactJSON(key.EffectiveRules()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
