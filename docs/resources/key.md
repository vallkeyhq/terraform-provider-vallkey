---
page_title: "vallkey_key Resource - Vallkey"
description: |-
  Create and manage a Vallkey ghost key and its access rules.
---

# vallkey_key (Resource)

Create a ghost key for a real provider credential. Account policies, safety delays and approval requirements continue to apply.

## Example usage

```terraform
variable "provider_key" {
  type      = string
  sensitive = true
}

resource "vallkey_key" "app" {
  service  = "openai"
  real_key = var.provider_key
  label    = "Application"
  rules = jsonencode({
    allow = [{ method = "GET", path = "/v1/models" }]
    ratePerMinute = 60
  })
}
```

## Schema

### Required

- `service` (String): Provider identifier. Changing it replaces the resource.
- `real_key` (String, Sensitive): Provider credential. Stored in Terraform state. Later changes replace the credential behind the ghost key, subject to account policy.

### Optional

- `label` (String): Display label; defaults to an empty string.
- `project` (String): Project grouping.
- `environment` (String): `production`, `staging`, or `development`.
- `rules` (String): JSON object, normally constructed with `jsonencode`. Omitted fields take provider defaults. If omitted entirely, Terraform does not manage rules.
- `custom` (String): JSON object for a custom provider, including `origin` and `auth`. Changing it replaces the resource.
- `aws` (String): JSON object containing `endpoint`, `region`, and `service`. Changing it replaces the resource.
- `skip_check` (Boolean): Allow creation or credential replacement despite a failed provider test request. Use only after understanding that failure.

### Read-only

- `id` (String): Key identifier.
- `ghost_key` (String, Sensitive): Ghost key returned at creation. Not recoverable for imported keys.
- `effective_rules` (String): Full desired rules, including defaults and pending looser rules. Pending rules are not necessarily active yet.
- `rules_apply_at` (String): Scheduled activation time in RFC 3339 format, or an empty string. Approval-pending changes may have no scheduled activation time.

## Changes and drift

Stricter rules apply immediately. Looser rules and credential replacement can wait for a safety delay or another administrator’s approval. A completed Terraform apply does not mean a pending change has activated; inspect its warning and the dashboard.

Changes made outside Terraform appear in subsequent plans when the corresponding setting is managed in configuration.

## Import

```sh
terraform import vallkey_key.app '<key-id>'
```

The original real and ghost keys cannot be retrieved. On an imported resource, the configured `real_key` is first adopted into state without comparison to the server’s secret. A subsequent configuration change triggers replacement of that secret. Terraform does not recover the imported ghost key.

## State security

Both `real_key` and `ghost_key` are stored in state. The sensitive flag controls display, not encryption. Restrict and encrypt state and plan storage.
