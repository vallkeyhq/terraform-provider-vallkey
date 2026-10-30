---
page_title: "Vallkey Provider"
description: |-
  Manage Vallkey ghost keys and access rules with Terraform.
---

# Vallkey Provider

Manage ghost keys and their rules through Vallkey’s management API. Obtain an account API token from Settings → API tokens and provide it through `VALLKEY_TOKEN`.

## Example usage

```terraform
terraform {
  required_providers {
    vallkey = { source = "vallkeyhq/vallkey" }
  }
}

provider "vallkey" {}
```

The registry source address applies after a signed provider release is published. For development before publication, build the provider and configure Terraform’s `dev_overrides`.

## Schema

### Optional

- `token` (String, Sensitive): Account API token. Defaults to `VALLKEY_TOKEN`; a token is required at configuration time.
- `api_url` (String): API origin. Defaults to `VALLKEY_API_URL`, then `https://api.vallkey.com`. Use the regional origin shown in your account when applicable. HTTPS is required except for loopback tests. Paths, query strings, fragments and embedded credentials are rejected.

## State security

The provider marks real and ghost keys as sensitive, but sensitive values remain in Terraform state. Use an encrypted state backend with restricted access. Avoid exporting state, plan files, or logs into public issues or repositories.
