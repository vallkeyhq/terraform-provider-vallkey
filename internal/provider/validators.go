package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type jsonObjectValidator struct{}

func jsonObject() validator.String { return jsonObjectValidator{} }

func (jsonObjectValidator) Description(context.Context) string { return "must be a JSON object" }
func (v jsonObjectValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (jsonObjectValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !ValidJSONObject(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Not a JSON object", "Write it with jsonencode({ ... }).")
	}
}

type oneOfValidator struct{ values []string }

func oneOf(values ...string) validator.String { return oneOfValidator{values} }

func (v oneOfValidator) Description(context.Context) string {
	return "must be one of: " + strings.Join(v.values, ", ")
}
func (v oneOfValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (v oneOfValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	for _, ok := range v.values {
		if req.ConfigValue.ValueString() == ok {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(req.Path, "Unknown value", fmt.Sprintf("%q %s", req.ConfigValue.ValueString(), v.Description(context.Background())))
}

// What the server says the rules are only changes when the configured rules do.
var rulesPath = path.Root("rules")

type sameRulesModifier struct{}

func sameRulesKeepState() planmodifier.String { return sameRulesModifier{} }

func (sameRulesModifier) Description(context.Context) string {
	return "keeps the stored value while the configured rules stay the same"
}
func (m sameRulesModifier) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }
func (sameRulesModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var planned, stored types.String
	if req.Plan.GetAttribute(ctx, rulesPath, &planned).HasError() || req.State.GetAttribute(ctx, rulesPath, &stored).HasError() {
		return
	}
	if planned.IsNull() || planned.Equal(stored) {
		resp.PlanValue = req.StateValue
	}
}
