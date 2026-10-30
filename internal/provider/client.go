package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to Vallkey's management API with an API token.
type Client struct {
	BaseURL   string
	Token     string
	HTTP      *http.Client
	UserAgent string
}

// APIError is an error answer from the API, with its code and message.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("vallkey: %s (%d %s)", e.Message, e.Status, e.Code)
	}
	return fmt.Sprintf("vallkey: %s (%d)", e.Message, e.Status)
}

// IsNotFound reports whether err is the API saying the thing does not exist.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

func NewClient(baseURL, token, version string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP: &http.Client{
			Timeout:       60 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
		UserAgent: "terraform-provider-vallkey/" + version,
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+"/_guard/v1"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		var e struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		msg := e.Error.Message
		if msg == "" {
			msg = http.StatusText(res.StatusCode)
		}
		return &APIError{Status: res.StatusCode, Code: e.Error.Code, Message: msg}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func validateAPIURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("use an HTTPS API origin without credentials, path, query or fragment")
	}
	loopback := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if (u.Scheme != "https" && !(u.Scheme == "http" && loopback)) || u.Hostname() == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return errors.New("use an HTTPS API origin without credentials, path, query or fragment; HTTP is only allowed for loopback tests")
	}
	return nil
}

// Key is a ghost key as the API describes it.
type Key struct {
	KeyID    string          `json:"keyId"`
	Provider string          `json:"provider"`
	Label    string          `json:"label"`
	Rules    json.RawMessage `json:"rules"`
	Pending  *struct {
		Rules       json.RawMessage `json:"rules"`
		ActivatesAt int64           `json:"activatesAt"`
	} `json:"pending"`
	Custom json.RawMessage `json:"custom"`
}

// EffectiveRules are the rules the key will have: waiting looser rules, or the current ones.
func (k *Key) EffectiveRules() json.RawMessage {
	if k.Pending != nil && len(k.Pending.Rules) > 0 {
		return k.Pending.Rules
	}
	return k.Rules
}

// KeyEntry is a key in the account's list, with its project and environment.
type KeyEntry struct {
	KeyID       string  `json:"keyId"`
	Label       string  `json:"label"`
	Provider    string  `json:"provider"`
	Project     *string `json:"project"`
	Environment *string `json:"environment"`
}

type CreateKeyInput struct {
	Provider    string          `json:"provider"`
	RealKey     string          `json:"real_key"`
	Label       string          `json:"label,omitempty"`
	Rules       json.RawMessage `json:"rules,omitempty"`
	Custom      json.RawMessage `json:"custom,omitempty"`
	AWS         json.RawMessage `json:"aws,omitempty"`
	Project     *string         `json:"project,omitempty"`
	Environment *string         `json:"environment,omitempty"`
	SkipCheck   bool            `json:"skip_check,omitempty"`
}

type CreatedKey struct {
	KeyID    string          `json:"key_id"`
	GhostKey string          `json:"ghost_key"`
	Rules    json.RawMessage `json:"rules"`
}

// ChangeResult says when a change applies: now, after the safety delay, or after a second admin's yes.
type ChangeResult struct {
	Applied     string `json:"applied"`
	ActivatesAt int64  `json:"activatesAt"`
}

func (c *Client) CreateKey(ctx context.Context, in CreateKeyInput) (*CreatedKey, error) {
	var out CreatedKey
	if err := c.do(ctx, http.MethodPost, "/keys", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetKey(ctx context.Context, id string) (*Key, error) {
	var out Key
	if err := c.do(ctx, http.MethodGet, "/keys/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) KeyEntry(ctx context.Context, id string) (*KeyEntry, error) {
	var out struct {
		Keys []KeyEntry `json:"keys"`
	}
	if err := c.do(ctx, http.MethodGet, "/keys", nil, &out); err != nil {
		return nil, err
	}
	for i := range out.Keys {
		if out.Keys[i].KeyID == id {
			return &out.Keys[i], nil
		}
	}
	return nil, &APIError{Status: http.StatusNotFound, Code: "not_found", Message: "no such key"}
}

// UpdateKey changes the label, project and environment; nil fields stay as they are.
func (c *Client) UpdateKey(ctx context.Context, id string, changes map[string]any) error {
	return c.do(ctx, http.MethodPatch, "/keys/"+id, changes, nil)
}

func (c *Client) UpdateRules(ctx context.Context, id string, rules json.RawMessage) (*ChangeResult, error) {
	var out ChangeResult
	if err := c.do(ctx, http.MethodPut, "/keys/"+id+"/rules", map[string]any{"rules": rules}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ReplaceRealKey(ctx context.Context, id, realKey string, skipCheck bool) (*ChangeResult, error) {
	var out ChangeResult
	body := map[string]any{"real_key": realKey}
	if skipCheck {
		body["skip_check"] = true
	}
	if err := c.do(ctx, http.MethodPut, "/keys/"+id+"/real-key", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteKey(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/keys/"+id, nil, nil)
}
