---
page_title: "Nile: nile_billing_customer"
subcategory: "Workspaces"
---

# nile_billing_customer (Resource)

Ensures a Stripe billing customer exists for a Nile workspace via `PUT
/workspaces/{workspaceSlug}/billing/customer`. The call is idempotent: it finds or creates the
linked Stripe customer. The Nile API offers no way to unlink a billing customer, so destroying
this resource only removes it from Terraform state. Billing endpoints require a session
(developer) token; API keys are rejected with 403.

## Example Usage

```terraform
resource "nile_billing_customer" "example" {
  workspace_slug = nile_workspace.example.slug
}
```

## Argument Reference

The following arguments are supported:

- `workspace_slug` - (Required) Slug of the workspace to ensure the billing customer for. Changing it forces replacement.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `default_payment_method` - (Computed) Default payment method attached to the Stripe customer, if any.
- `raw_json` - (Computed) Raw API response of the billing customer object, as JSON (secret fields redacted).
- `stripe_customer_id` - (Computed) Stripe customer identifier linked to the workspace.

## Notes

### Idempotent ensure

Create, read and update all issue the same idempotent call that finds or
creates the Stripe customer for the workspace. The API cannot unlink a billing
customer, so destroy only removes the resource from state.

## Import

Import an existing object with:

```sh
terraform import nile_billing_customer.example my-workspace
```

