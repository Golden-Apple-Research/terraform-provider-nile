// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

// Command gendocs generates the registry documentation under docs/ from the
// provider schemas, so the docs cannot drift from the implementation.
//
// Usage (from the repository root):
//
//	go run ./tools/gendocs
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
	"github.com/Golden-Apple-Research/nile-terraform/internal/provider"
)

// attribute is the common subset of resource and data source schema
// attributes used for rendering.
type attribute interface {
	GetMarkdownDescription() string
	IsRequired() bool
	IsOptional() bool
	IsComputed() bool
	IsSensitive() bool
}

type target struct {
	name        string
	subcategory string
}

var resourceTargets = []struct {
	target
	constructor func() resource.Resource
}{
	{target{"database", "Databases"}, provider.NewDatabaseResource},
	{target{"database_compute_instance", "Databases"}, provider.NewComputeInstanceResource},
	{target{"database_credential", "Databases"}, provider.NewDatabaseCredentialResource},
	{target{"developer_invite", "Developers"}, provider.NewDeveloperInviteResource},
	{target{"workspace", "Workspaces"}, provider.NewWorkspaceResource},
	{target{"workspace_subscription", "Workspaces"}, provider.NewWorkspaceSubscriptionResource},
	{target{"billing_customer", "Workspaces"}, provider.NewBillingCustomerResource},
	{target{"provisioned_database", "Databases"}, provider.NewProvisionedDatabaseResource},
}

var dataSourceTargets = []struct {
	target
	constructor func() datasource.DataSource
}{
	{target{"database", "Databases"}, provider.NewDatabaseDataSource},
	{target{"databases", "Databases"}, provider.NewDatabasesDataSource},
	{target{"database_compute_instances", "Databases"}, provider.NewDatabaseComputeInstancesDataSource},
	{target{"database_credentials", "Databases"}, provider.NewDatabaseCredentialsDataSource},
	{target{"database_uptime_insights", "Databases"}, provider.NewDatabaseUptimeInsightsDataSource},
	{target{"database_error_insights", "Databases"}, provider.NewDatabaseErrorInsightsDataSource},
	{target{"database_query_performance_insights", "Databases"}, provider.NewDatabaseQueryPerformanceInsightsDataSource},
	{target{"compute_types", "Workspaces"}, provider.NewComputeTypesDataSource},
	{target{"regions", "Workspaces"}, provider.NewRegionsDataSource},
	{target{"workspace", "Workspaces"}, provider.NewWorkspaceDataSource},
	{target{"workspaces", "Workspaces"}, provider.NewWorkspacesDataSource},
	{target{"workspace_compute_usage", "Workspaces"}, provider.NewWorkspaceComputeUsageDataSource},
	{target{"workspace_developers", "Workspaces"}, provider.NewWorkspaceDevelopersDataSource},
	{target{"workspace_invites", "Developers"}, provider.NewWorkspaceInvitesDataSource},
	{target{"workspace_subscription", "Workspaces"}, provider.NewWorkspaceSubscriptionDataSource},
	{target{"workspace_subscription_history", "Workspaces"}, provider.NewWorkspaceSubscriptionHistoryDataSource},
	{target{"workspace_billing_readiness", "Workspaces"}, provider.NewWorkspaceBillingReadinessDataSource},
	{target{"workspace_billing_totals", "Workspaces"}, provider.NewWorkspaceBillingTotalsDataSource},
	{target{"developer", "Developers"}, provider.NewDeveloperDataSource},
}

// codeSpanMark stands in for a backtick inside the raw-string literals in
// notes. render replaces it before writing, which keeps the note literals free
// of string concatenation while still producing Markdown inline code.
const codeSpanMark = "§"

// notes holds extra prose appended as a "Notes" section, keyed by
// "<docs dir>/<name>". Use it for operational behavior that is not part of the
// schema (asynchronous waits, one-time secrets, pagination, retries) and for
// cross-links to related pages. Links must carry the .md extension so they
// resolve on GitHub as well as in the Terraform Registry.
var notes = map[string]string{
	"resources/database": `### Asynchronous lifecycle

Creating a database and renaming it (§name§) are asynchronous. The resource
polls the API until the database reports §READY§ before it completes, so
dependent resources such as [nile_database_compute_instance](database_compute_instance.md)
and [nile_database_credential](database_credential.md) can be created in the same
apply. Waiting is bounded by the §create§ and §update§ timeouts; deletion is
bounded by the §delete§ timeout.

### Partial success

If the API has already created or renamed the database when a later wait fails,
the resource is still recorded in state, so a subsequent refresh can adopt it
instead of orphaning it.`,
	"resources/database_compute_instance": `### Asynchronous lifecycle

Creating, resizing (§instance_size§) and deleting an instance are asynchronous:
the resource waits until the instance reports §READY§ (or disappears on delete)
before it completes. Renaming (§instance_name§) updates the instance in place.

### Discovering sizes

Use the [nile_compute_types](../data-sources/compute_types.md) data source to
list valid §instance_size§ values for a workspace, and the
[nile_database_compute_instances](../data-sources/database_compute_instances.md)
data source to inspect the instances that already exist.`,
	"resources/database_credential": `### One-time password

The API returns the credential password exactly once, in the create response. It
is stored in state as a sensitive value and is never copied from later responses.

### Rotation

The API has no credential update endpoint, so changing §tenant_id§ or §internal§
forces replacement. To rotate the password in place, use the §RotateCredential§
client method or the Nile API directly.`,
	"resources/developer_invite": `### Email vs. programmatic invites

With §programmatic = true§ the API returns a one-time invite code in the sensitive
§code§ attribute instead of only sending an email. The code is returned once and
never copied from later responses.

### Replacement

There is no invite update endpoint, so changing §email§ or §programmatic§ forces
replacement.`,
	"data-sources/database_compute_instances": `### Time window

Set §start§ and §end§ to RFC3339 timestamps to restrict the result to instances
that were active during that period. §start§ must not be later than §end§; the
provider rejects an inverted window before making any API call.

### Pagination

The Nile API currently returns all matching instances in a single response. If it
ever introduces pagination (a response object carrying a continuation token such
as §nextPageToken§), the provider follows the token automatically and concatenates
all pages, so §instances§ always contains the complete result. Safety guards abort
with an error if a server fails to advance its page tokens.

### Retries

Transient failures — HTTP §408§, §429§, §5xx§, and network errors — are retried up
to three times with exponential backoff and jitter for replay-safe methods. A
§Retry-After§ response header takes precedence over the computed backoff (capped
at 30 seconds). Client errors such as §400§ or §401§ are never retried.`,
	"data-sources/workspace_billing_readiness": `This data source only inspects the workspace's billing state. It never creates
a billing customer; use the §nile_billing_customer§ resource for that.`,
	"resources/workspace": `### Session-token authentication

Creating a workspace is rejected with 403 §forbidden_operation§ when the
provider authenticates with an API key; use OAuth credentials
(§oauth_client_id§/§oauth_refresh_token§) or a session token instead.

### Deletion is state-only

The Nile API does not expose workspace deletion. Destroying this resource
removes it from Terraform state but leaves the workspace running in the
control plane.`,
	"resources/workspace_subscription": `### Level changes

Changing §level§ updates the subscription in place via the change endpoint.
Destroying the resource closes the subscription. Billing endpoints require a
session (developer) token; API keys are rejected with 403.`,
	"resources/billing_customer": `### Idempotent ensure

Create, read and update all issue the same idempotent call that finds or
creates the Stripe customer for the workspace. The API cannot unlink a billing
customer, so destroy only removes the resource from state.`,
	"resources/provisioned_database": `### Claim flow

The provisioning call is unauthenticated and returns a one-time claim code.
Consume it with the §claim_code§ attribute of §nile_database§ to attach the
dedicated database to a workspace. Dedicated compute requires a paid plan; the
free tier answers with 403. No read or delete endpoint exists for provisioned
databases, so refresh and destroy perform no API call.`,
}

// importIDs maps resource names to their terraform import identifier format.
var importIDs = map[string]string{
	"database":                  "my-workspace/app-database",
	"database_compute_instance": "my-workspace/app-database/inst-abc123",
	"database_credential":       "my-workspace/app-database/cred-abc123",
	"developer_invite":          "my-workspace/inv-abc123",
	"workspace":                 "my-workspace",
	"workspace_subscription":    "my-workspace",
	"billing_customer":          "my-workspace",
}

// examples holds a short HCL block per object, keyed by "<docs dir>/<name>".
var examples = map[string]string{
	"resources/database": `resource "nile_database" "example" {
  workspace_slug = "my-workspace"
  name           = "app-database"
  region         = "AWS_US_WEST_2"
}`,
	"resources/database_compute_instance": `resource "nile_database_compute_instance" "example" {
  workspace_slug = "my-workspace"
  database_name  = "app-database"
  instance_name  = "primary"
  instance_size  = "large"
}`,
	"resources/database_credential": `resource "nile_database_credential" "example" {
  workspace_slug = "my-workspace"
  database_name  = "app-database"
}`,
	"resources/developer_invite": `resource "nile_developer_invite" "example" {
  workspace_slug = "my-workspace"
  email          = "qa@example.com"
  programmatic   = true
}`,
	"resources/workspace": `resource "nile_workspace" "example" {
  name = "Research"
}`,
	"resources/workspace_subscription": `resource "nile_workspace_subscription" "example" {
  workspace_slug = nile_workspace.example.slug
  level          = "paid"
}`,
	"resources/billing_customer": `resource "nile_billing_customer" "example" {
  workspace_slug = nile_workspace.example.slug
}`,
	"resources/provisioned_database": `resource "nile_provisioned_database" "example" {
  region = "AWS_EU_CENTRAL_1"
}

resource "nile_database" "claimed" {
  workspace_slug = nile_workspace.example.slug
  region         = "AWS_EU_CENTRAL_1"
  claim_code     = nile_provisioned_database.example.claim_code
}`,
	"data-sources/database": `data "nile_database" "example" {
  workspace_slug = "my-workspace"
  name           = "app-database"
}`,
	"data-sources/databases": `data "nile_databases" "example" {
  workspace_slug = "my-workspace"
}`,
	"data-sources/database_credentials": `data "nile_database_credentials" "example" {
  workspace_slug = "my-workspace"
  database_name  = "app-database"
}`,
	"data-sources/database_compute_instances": `data "nile_database_compute_instances" "example" {
  workspace_slug = "my-workspace"
  database_name  = "app-database"
}`,
	"data-sources/database_uptime_insights": `data "nile_database_uptime_insights" "example" {
  workspace_slug = "my-workspace"
  database       = "app-database"
  start          = "2025-06-01T00:00:00Z"
  end            = "2025-06-02T00:00:00Z"
  granularity    = "1h"
}`,
	"data-sources/database_error_insights": `data "nile_database_error_insights" "example" {
  workspace_slug = "my-workspace"
  database       = "app-database"
  granularity    = "5m"
}`,
	"data-sources/database_query_performance_insights": `data "nile_database_query_performance_insights" "example" {
  workspace_slug = "my-workspace"
  database       = "app-database"
}`,
	"data-sources/compute_types": `data "nile_compute_types" "example" {
  workspace_slug = "my-workspace"
}`,
	"data-sources/regions": `data "nile_regions" "example" {
  workspace_slug = "my-workspace"
}`,
	"data-sources/workspace": `data "nile_workspace" "example" {
  slug = "my-workspace"
}`,
	"data-sources/workspaces": `data "nile_workspaces" "example" {}`,
	"data-sources/workspace_compute_usage": `data "nile_workspace_compute_usage" "example" {
  workspace_slug = "my-workspace"
  start          = "2025-06-01T00:00:00Z"
  end            = "2025-07-01T00:00:00Z"
}`,
	"data-sources/workspace_developers": `data "nile_workspace_developers" "example" {
  workspace_slug = "my-workspace"
}`,
	"data-sources/workspace_invites": `data "nile_workspace_invites" "example" {
  workspace_slug = "my-workspace"
}`,
	"data-sources/workspace_subscription": `data "nile_workspace_subscription" "example" {
  workspace_slug = "my-workspace"
}`,
	"data-sources/workspace_subscription_history": `data "nile_workspace_subscription_history" "example" {
  workspace_slug = "my-workspace"
}`,
	"data-sources/workspace_billing_readiness": `data "nile_workspace_billing_readiness" "example" {
  workspace_slug = "my-workspace"
}`,
	"data-sources/workspace_billing_totals": `data "nile_workspace_billing_totals" "example" {
  workspace_slug = "my-workspace"
  month          = "2025-06"
}`,
	"data-sources/developer": `data "nile_developer" "example" {}`,
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	for _, t := range resourceTargets {
		r := t.constructor()
		var mResp resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "nile"}, &mResp)
		var sResp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &sResp)
		if sResp.Diagnostics.HasError() {
			return fmt.Errorf("resource %s schema: %v", mResp.TypeName, sResp.Diagnostics)
		}
		doc := render("resources", "Resource", mResp.TypeName, sResp.Schema.MarkdownDescription, convertNested(sResp.Schema.Attributes), importIDs[t.name])
		if err := writeDoc(filepath.Join("docs", "resources", t.name+".md"), mResp.TypeName, t.subcategory, doc); err != nil {
			return err
		}
	}
	for _, t := range dataSourceTargets {
		d := t.constructor()
		var mResp datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "nile"}, &mResp)
		var sResp datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &sResp)
		if sResp.Diagnostics.HasError() {
			return fmt.Errorf("data source %s schema: %v", mResp.TypeName, sResp.Diagnostics)
		}
		doc := render("data-sources", "Data Source", mResp.TypeName, sResp.Schema.MarkdownDescription, convertNested(sResp.Schema.Attributes), "")
		if err := writeDoc(filepath.Join("docs", "data-sources", t.name+".md"), mResp.TypeName, t.subcategory, doc); err != nil {
			return err
		}
	}
	fmt.Println("generated docs for", len(resourceTargets)+len(dataSourceTargets), "objects")
	return nil
}

func writeDoc(path, typeName, subcategory, body string) error {
	content := fmt.Sprintf("---\npage_title: \"Nile: %s\"\nsubcategory: \"%s\"\n---\n\n%s\n", typeName, subcategory, body)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// render turns a schema into the markdown sections of a docs page. dir is the
// docs subdirectory ("resources" or "data-sources").
func render(dir, kind, typeName, description string, attrs map[string]attribute, importID string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%s)\n\n%s\n", typeName, kind, wrapDescription(description))

	example := examples[dir+"/"+shortName(typeName)]
	if example != "" {
		b.WriteString("\n## Example Usage\n\n```terraform\n")
		b.WriteString(example)
		b.WriteString("\n```\n")
	}

	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)

	// Arguments are everything a practitioner may configure: required and
	// optional attributes, including optional-and-computed ones. Attributes are
	// the values the provider exports. An optional-and-computed attribute is
	// documented in both sections, matching the Terraform Registry convention.
	arguments := make([]string, 0, len(names))
	attributes := make([]string, 0, len(names))
	for _, name := range names {
		if attrs[name].IsRequired() || attrs[name].IsOptional() {
			arguments = append(arguments, name)
		}
		if attrs[name].IsComputed() {
			attributes = append(attributes, name)
		}
	}

	b.WriteString("\n## Argument Reference\n\n")
	if len(arguments) == 0 {
		b.WriteString("This object has no arguments.\n")
	} else {
		b.WriteString("The following arguments are supported:\n\n")
		for _, name := range arguments {
			writeAttribute(&b, attrs[name], name, "", "argument")
		}
	}

	b.WriteString("\n## Attribute Reference\n\n")
	switch {
	case len(attributes) == 0:
		b.WriteString("This object has no additional attributes.\n")
	case len(arguments) == 0:
		b.WriteString("The following attributes are exported:\n\n")
	default:
		b.WriteString("In addition to the arguments above, the following attributes are exported:\n\n")
	}
	for _, name := range attributes {
		writeAttribute(&b, attrs[name], name, "", "attribute")
	}

	if note := notes[dir+"/"+shortName(typeName)]; note != "" {
		b.WriteString("\n## Notes\n\n")
		b.WriteString(strings.ReplaceAll(strings.TrimSpace(note), codeSpanMark, "`") + "\n")
	}

	if importID != "" {
		fmt.Fprintf(&b, "\n## Import\n\nImport an existing object with:\n\n```sh\nterraform import %s.example %s\n```\n", typeName, importID)
	}
	return b.String()
}

// writeAttribute renders one attribute and, recursively, its nested attributes.
// section is "argument" or "attribute" and selects the mode label: an
// optional-and-computed attribute is "Optional" where it can be configured and
// "Computed" where it is exported.
func writeAttribute(b *strings.Builder, a attribute, name, indent, section string) {
	mode := "Computed"
	if section == "argument" {
		if a.IsRequired() {
			mode = "Required"
		} else {
			mode = "Optional"
		}
	}
	if a.IsSensitive() {
		mode += ", Sensitive"
	}
	// The framework's timeouts package ships its attributes without
	// descriptions; fill them in so the generated docs are self-explanatory.
	description := strings.TrimSpace(timeoutsDescription(name, a.GetMarkdownDescription()))
	line := fmt.Sprintf("%s- `%s` - (%s)", indent, name, mode)
	if description != "" {
		line += " " + description
	}
	b.WriteString(line + "\n")
	if nested := nestedAttributes(a); nested != nil {
		nestedNames := make([]string, 0, len(nested))
		for n := range nested {
			nestedNames = append(nestedNames, n)
		}
		sort.Strings(nestedNames)
		for _, n := range nestedNames {
			writeAttribute(b, nested[n], n, indent+"  ", section)
		}
	}
}

// defaultTimeoutDoc renders the provider's default wait timeout for the
// generated docs (for example "20m").
func defaultTimeoutDoc() string {
	d := nileapi.DefaultWaitTimeout
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return d.String()
}

// timeoutsDescription returns a documentation string for the framework's
// timeouts attributes, which carry no descriptions of their own. Any other
// attribute passes its own description through unchanged.
func timeoutsDescription(name, description string) string {
	if description != "" {
		return description
	}
	def := defaultTimeoutDoc()
	switch name {
	case "timeouts":
		return "Timeouts for asynchronous operations. Unspecified operations use the default of `" + def + "`."
	case "create":
		return "Time to wait for the resource to be created and become ready. Defaults to `" + def + "`."
	case "read":
		return "Time to wait for the resource to be read."
	case "update":
		return "Time to wait for the resource update to settle. Defaults to `" + def + "`."
	case "delete":
		return "Time to wait for the resource to be deleted. Defaults to `" + def + "`."
	}
	return ""
}

// nestedAttributes returns the child attributes of a nested attribute, or nil.
func nestedAttributes(a attribute) map[string]attribute {
	switch v := a.(type) {
	case rschema.ListNestedAttribute:
		return convertNested(v.NestedObject.Attributes)
	case rschema.SetNestedAttribute:
		return convertNested(v.NestedObject.Attributes)
	case rschema.SingleNestedAttribute:
		return convertNested(v.Attributes)
	case rschema.MapNestedAttribute:
		return convertNested(v.NestedObject.Attributes)
	case dsschema.ListNestedAttribute:
		return convertNested(v.NestedObject.Attributes)
	case dsschema.SetNestedAttribute:
		return convertNested(v.NestedObject.Attributes)
	case dsschema.SingleNestedAttribute:
		return convertNested(v.Attributes)
	case dsschema.MapNestedAttribute:
		return convertNested(v.NestedObject.Attributes)
	}
	return nil
}

// convertNested widens a concrete attribute map to the attribute interface.
// Map types are invariant in Go, so the elements have to be copied.
func convertNested[A attribute](in map[string]A) map[string]attribute {
	out := make(map[string]attribute, len(in))
	for name, a := range in {
		out[name] = a
	}
	return out
}

func shortName(typeName string) string {
	return strings.TrimPrefix(typeName, "nile_")
}

func wrapDescription(description string) string {
	const maxWidth = 96
	words := strings.Fields(description)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > maxWidth {
			lines = append(lines, line)
			line = word
			continue
		}
		line += " " + word
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}
