package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &keyResource{}
	_ resource.ResourceWithConfigure   = &keyResource{}
	_ resource.ResourceWithImportState = &keyResource{}
)

func NewKeyResource() resource.Resource { return &keyResource{} }

type keyResource struct {
	client *Client
}

type keyModel struct {
	ID             types.String `tfsdk:"id"`
	Service        types.String `tfsdk:"service"`
	RealKey        types.String `tfsdk:"real_key"`
	SkipCheck      types.Bool   `tfsdk:"skip_check"`
	Label          types.String `tfsdk:"label"`
	Project        types.String `tfsdk:"project"`
	Environment    types.String `tfsdk:"environment"`
	Rules          types.String `tfsdk:"rules"`
	Custom         types.String `tfsdk:"custom"`
	AWS            types.String `tfsdk:"aws"`
	GhostKey       types.String `tfsdk:"ghost_key"`
	EffectiveRules types.String `tfsdk:"effective_rules"`
	RulesApplyAt   types.String `tfsdk:"rules_apply_at"`
}

func (r *keyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_key"
}

func (r *keyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A ghost key: the key your app uses in place of a real API key, with the rules Vallkey checks on every request.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The key's id.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"service": schema.StringAttribute{
				Required:      true,
				Description:   "The provider: openai, anthropic, stripe, github, custom, aws, and so on (GET /_guard/v1/providers lists them). Changing it makes a new key.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"real_key": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				Description: "The real API key. Vallkey encrypts it and never shows it again. Changing it replaces the real key behind the same ghost key, after the account's safety delay. " +
					"Terraform keeps it in state: use a state backend that encrypts.",
			},
			"skip_check": schema.BoolAttribute{
				Optional:    true,
				Description: "Create the key even if the provider refuses the real key in Vallkey's one test request.",
			},
			"label": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "A name for the key.",
			},
			"project": schema.StringAttribute{
				Optional:    true,
				Description: "The project the key belongs to.",
			},
			"environment": schema.StringAttribute{
				Optional:    true,
				Description: "production, staging or development. Production keys must meet the account's production policies.",
				Validators:  []validator.String{oneOf("production", "staging", "development")},
			},
			"rules": schema.StringAttribute{
				Optional: true,
				Description: "The rules, as JSON (use jsonencode). Fields left out take the provider's defaults. Stricter rules apply at once; looser ones after the account's safety delay, " +
					"or a second admin's approval. Without it, Terraform leaves the rules alone.",
				Validators: []validator.String{jsonObject()},
			},
			"custom": schema.StringAttribute{
				Optional:      true,
				Description:   "For service = \"custom\": the API's address and how the key is sent, as JSON: { origin, auth, preset }. Changing it makes a new key.",
				Validators:    []validator.String{jsonObject()},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"aws": schema.StringAttribute{
				Optional:      true,
				Description:   "For service = \"aws\": { endpoint, region, service }, as JSON. Changing it makes a new key.",
				Validators:    []validator.String{jsonObject()},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ghost_key": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "The ghost key for your app. Vallkey shows it only once, when the key is made; it is empty for imported keys.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"effective_rules": schema.StringAttribute{
				Computed:      true,
				Description:   "Every rule as Vallkey holds it, defaults included, as JSON: the looser rules waiting to apply, or the current ones.",
				PlanModifiers: []planmodifier.String{sameRulesKeepState()},
			},
			"rules_apply_at": schema.StringAttribute{
				Computed:      true,
				Description:   "When looser rules waiting out the safety delay apply (RFC 3339), or empty.",
				PlanModifiers: []planmodifier.String{sameRulesKeepState()},
			},
		},
	}
}

func (r *keyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func optionalJSON(v types.String) json.RawMessage {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return json.RawMessage(v.ValueString())
}

func optionalString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

func (r *keyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan keyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateKey(ctx, CreateKeyInput{
		Provider:    plan.Service.ValueString(),
		RealKey:     plan.RealKey.ValueString(),
		Label:       plan.Label.ValueString(),
		Rules:       optionalJSON(plan.Rules),
		Custom:      optionalJSON(plan.Custom),
		AWS:         optionalJSON(plan.AWS),
		Project:     optionalString(plan.Project),
		Environment: optionalString(plan.Environment),
		SkipCheck:   plan.SkipCheck.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Could not create the key", err.Error())
		return
	}
	plan.ID = types.StringValue(created.KeyID)
	plan.GhostKey = types.StringValue(created.GhostKey)
	plan.EffectiveRules = types.StringValue(CompactJSON(created.Rules))
	plan.RulesApplyAt = types.StringValue("")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *keyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state keyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key, err := r.client.GetKey(ctx, state.ID.ValueString())
	if IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not read the key", err.Error())
		return
	}
	entry, err := r.client.KeyEntry(ctx, state.ID.ValueString())
	if err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Could not read the key's project", err.Error())
		return
	}
	applyKey(&state, key, entry)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// applyKey brings the state in line with the key as Vallkey holds it. Rules changed
// outside Terraform show as a difference against the configured rules.
func applyKey(state *keyModel, key *Key, entry *KeyEntry) {
	effective := key.EffectiveRules()
	// Imported keys have no rules in state: they are not managed until a configuration names them.
	if !state.Rules.IsNull() && !state.EffectiveRules.IsNull() && !SameJSON(effective, []byte(state.EffectiveRules.ValueString())) {
		state.Rules = types.StringValue(CompactJSON(effective))
	}
	state.EffectiveRules = types.StringValue(CompactJSON(effective))
	state.RulesApplyAt = types.StringValue("")
	if key.Pending != nil && key.Pending.ActivatesAt > 0 {
		state.RulesApplyAt = types.StringValue(time.UnixMilli(key.Pending.ActivatesAt).UTC().Format(time.RFC3339))
	}
	state.Service = types.StringValue(key.Provider)
	state.Label = types.StringValue(key.Label)
	if entry != nil {
		state.Project = nullableString(entry.Project)
		state.Environment = nullableString(entry.Environment)
	}
	if !state.Custom.IsNull() && len(key.Custom) > 0 && string(key.Custom) != "null" && !SameJSON(key.Custom, []byte(state.Custom.ValueString())) {
		// The server fills in what was left out; only a real difference shows.
		var given, held map[string]any
		_ = json.Unmarshal([]byte(state.Custom.ValueString()), &given)
		_ = json.Unmarshal(key.Custom, &held)
		if given["origin"] != held["origin"] {
			state.Custom = types.StringValue(CompactJSON(key.Custom))
		}
	}
	if state.GhostKey.IsNull() || state.GhostKey.IsUnknown() {
		state.GhostKey = types.StringValue("")
	}
}

func nullableString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

func (r *keyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state keyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	changes := map[string]any{}
	if !plan.Label.Equal(state.Label) {
		changes["label"] = plan.Label.ValueString()
	}
	if !plan.Project.Equal(state.Project) {
		changes["project"] = optionalString(plan.Project)
	}
	if !plan.Environment.Equal(state.Environment) {
		changes["environment"] = optionalString(plan.Environment)
	}
	if len(changes) > 0 {
		if err := r.client.UpdateKey(ctx, id, changes); err != nil {
			resp.Diagnostics.AddError("Could not update the key", err.Error())
			return
		}
	}

	if !plan.Rules.IsNull() && !plan.Rules.Equal(state.Rules) {
		result, err := r.client.UpdateRules(ctx, id, optionalJSON(plan.Rules))
		if err != nil {
			resp.Diagnostics.AddError("Could not update the rules", err.Error())
			return
		}
		warnChange(resp, "The looser rules", result)
	}

	// An imported key's real key is not known, so the configured one is taken as it is.
	if !state.RealKey.IsNull() && !plan.RealKey.Equal(state.RealKey) {
		result, err := r.client.ReplaceRealKey(ctx, id, plan.RealKey.ValueString(), plan.SkipCheck.ValueBool())
		if err != nil {
			resp.Diagnostics.AddError("Could not replace the real key", err.Error())
			return
		}
		warnChange(resp, "The new real key", result)
	}

	key, err := r.client.GetKey(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Could not read the key", err.Error())
		return
	}
	plan.ID = state.ID
	plan.GhostKey = state.GhostKey
	plan.EffectiveRules = types.StringValue(CompactJSON(key.EffectiveRules()))
	plan.RulesApplyAt = types.StringValue("")
	if key.Pending != nil && key.Pending.ActivatesAt > 0 {
		plan.RulesApplyAt = types.StringValue(time.UnixMilli(key.Pending.ActivatesAt).UTC().Format(time.RFC3339))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Changes that loosen a key wait: Terraform finishes, and says when they apply.
func warnChange(resp *resource.UpdateResponse, what string, result *ChangeResult) {
	switch result.Applied {
	case "scheduled":
		resp.Diagnostics.AddWarning(what+" wait out the safety delay", fmt.Sprintf("They apply at %s. Your team was alerted.", time.UnixMilli(result.ActivatesAt).UTC().Format(time.RFC3339)))
	case "approval":
		resp.Diagnostics.AddWarning(what+" need another admin's approval", "Approve it in the dashboard, on the key's page. Until then nothing changes.")
	}
}

func (r *keyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state keyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteKey(ctx, state.ID.ValueString()); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Could not delete the key", err.Error())
	}
}

func (r *keyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
