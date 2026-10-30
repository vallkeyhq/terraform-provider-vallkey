---
page_title: "vallkey_key Data Source - Vallkey"
description: |-
  Read a Vallkey ghost key’s metadata and desired rules without retrieving credentials.
---

# vallkey_key (Data Source)

Read metadata and desired rules for an existing ghost key. Real and ghost key credentials are not returned.

## Example usage

```terraform
data "vallkey_key" "existing" {
  id = var.vallkey_key_id
}
```

## Schema

### Required

- `id` (String): Existing key identifier.

### Read-only

- `service` (String): Provider identifier.
- `label` (String): Display label.
- `project` (String): Project grouping, if set.
- `environment` (String): Deployment environment, if set.
- `rules` (String): Full desired rules as JSON, including pending looser rules where present. These rules may not yet be active; inspect pending changes in the dashboard.
