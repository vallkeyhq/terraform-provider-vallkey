package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSchemasAreValid(t *testing.T) {
	ctx := context.Background()
	p := New("test")()
	var ps fwprovider.SchemaResponse
	p.Schema(ctx, fwprovider.SchemaRequest{}, &ps)
	if ps.Diagnostics.HasError() {
		t.Fatal(ps.Diagnostics)
	}
	var rs resource.SchemaResponse
	NewKeyResource().Schema(ctx, resource.SchemaRequest{}, &rs)
	if d := rs.Schema.ValidateImplementation(ctx); d.HasError() {
		t.Fatal(d)
	}
	if !rs.Schema.Attributes["real_key"].IsSensitive() || !rs.Schema.Attributes["ghost_key"].IsSensitive() {
		t.Fatal("real_key and ghost_key must be sensitive")
	}
	var ds datasource.SchemaResponse
	NewKeyDataSource().Schema(ctx, datasource.SchemaRequest{}, &ds)
	if d := ds.Schema.ValidateImplementation(ctx); d.HasError() {
		t.Fatal(d)
	}
}

// A fake management API that records what it was asked.
type fakeAPI struct {
	calls  []string
	bodies []map[string]any
}

func (f *fakeAPI) server(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gk_api_token" {
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"error":{"code":"unauthorized","message":"invalid token"}}`)
			return
		}
		f.calls = append(f.calls, r.Method+" "+r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.bodies = append(f.bodies, body)
		switch {
		case r.Method == "POST" && r.URL.Path == "/_guard/v1/keys":
			w.WriteHeader(201)
			_, _ = io.WriteString(w, `{"key_id":"abcdefghijklmnopqrstuv","ghost_key":"gk_openai_x","rules":{"allow":[],"ratePerMinute":60}}`)
		case r.Method == "GET" && r.URL.Path == "/_guard/v1/keys/abcdefghijklmnopqrstuv":
			_, _ = io.WriteString(w, `{"keyId":"abcdefghijklmnopqrstuv","provider":"openai","label":"App","rules":{"allow":[],"ratePerMinute":60},"pending":{"rules":{"allow":[],"ratePerMinute":120},"activatesAt":1790000000000},"custom":null}`)
		case r.Method == "GET" && r.URL.Path == "/_guard/v1/keys":
			_, _ = io.WriteString(w, `{"keys":[{"keyId":"abcdefghijklmnopqrstuv","label":"App","provider":"openai","project":"web","environment":null}]}`)
		case r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/rules"):
			_, _ = io.WriteString(w, `{"applied":"scheduled","activatesAt":1790000000000}`)
		case r.Method == "DELETE":
			w.WriteHeader(404)
			_, _ = io.WriteString(w, `{"error":{"code":"not_found","message":"no such key"}}`)
		default:
			_, _ = io.WriteString(w, `{}`)
		}
	}))
}

func TestClient(t *testing.T) {
	ctx := context.Background()
	api := &fakeAPI{}
	srv := api.server(t)
	defer srv.Close()
	c := NewClient(srv.URL+"/", "gk_api_token", "test")

	created, err := c.CreateKey(ctx, CreateKeyInput{Provider: "openai", RealKey: "sk-real", Rules: json.RawMessage(`{"allow":[]}`)})
	if err != nil || created.KeyID != "abcdefghijklmnopqrstuv" || created.GhostKey != "gk_openai_x" {
		t.Fatalf("create: %v %+v", err, created)
	}
	if api.bodies[0]["real_key"] != "sk-real" || api.bodies[0]["provider"] != "openai" {
		t.Fatalf("create body: %v", api.bodies[0])
	}
	if _, ok := api.bodies[0]["custom"]; ok {
		t.Fatal("custom must be left out when not set")
	}

	key, err := c.GetKey(ctx, created.KeyID)
	if err != nil || !SameJSON(key.EffectiveRules(), []byte(`{"ratePerMinute":120,"allow":[]}`)) {
		t.Fatalf("effective rules should be the waiting ones: %v %s", err, key.EffectiveRules())
	}
	entry, err := c.KeyEntry(ctx, created.KeyID)
	if err != nil || entry.Project == nil || *entry.Project != "web" || entry.Environment != nil {
		t.Fatalf("entry: %v %+v", err, entry)
	}
	result, err := c.UpdateRules(ctx, created.KeyID, json.RawMessage(`{"ratePerMinute":120}`))
	if err != nil || result.Applied != "scheduled" {
		t.Fatalf("rules: %v %+v", err, result)
	}
	if err := c.DeleteKey(ctx, created.KeyID); !IsNotFound(err) {
		t.Fatalf("a missing key is not found: %v", err)
	}
	bad := NewClient(srv.URL, "wrong", "test")
	if _, err := bad.GetKey(ctx, "x"); err == nil || !strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("error message should come through: %v", err)
	}
}

func TestDriftShowsAgainstConfiguredRules(t *testing.T) {
	key := &Key{KeyID: "k", Provider: "openai", Label: "App", Rules: json.RawMessage(`{"allow":[],"ratePerMinute":60}`)}
	// Nothing changed outside Terraform: the configured rules stay as written.
	state := keyModel{Rules: types.StringValue(`{"ratePerMinute":60}`), EffectiveRules: types.StringValue(`{"ratePerMinute":60,"allow":[]}`), Custom: types.StringNull()}
	applyKey(&state, key, nil)
	if state.Rules.ValueString() != `{"ratePerMinute":60}` {
		t.Fatalf("no drift expected, got %s", state.Rules.ValueString())
	}
	// Someone loosened the rules in the dashboard: Terraform sees it.
	key.Rules = json.RawMessage(`{"allow":[],"ratePerMinute":600}`)
	applyKey(&state, key, nil)
	if !SameJSON([]byte(state.Rules.ValueString()), key.Rules) {
		t.Fatalf("drift should show, got %s", state.Rules.ValueString())
	}
	// Unmanaged rules stay unmanaged.
	free := keyModel{Rules: types.StringNull(), EffectiveRules: types.StringNull(), Custom: types.StringNull()}
	applyKey(&free, key, nil)
	if !free.Rules.IsNull() || free.EffectiveRules.IsNull() {
		t.Fatal("rules not in the configuration must stay unmanaged")
	}
}

func TestSameJSON(t *testing.T) {
	if !SameJSON([]byte(`{"a":1,"b":[1,2]}`), []byte(`{ "b": [1,2], "a": 1 }`)) {
		t.Fatal("key order and spacing do not matter")
	}
	if SameJSON([]byte(`{"a":1}`), []byte(`{"a":2}`)) || SameJSON([]byte(`{`), []byte(`{`)) {
		t.Fatal("different or invalid documents are not the same")
	}
}
