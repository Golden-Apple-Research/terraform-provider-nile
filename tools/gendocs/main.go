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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

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
}

var dataSourceTargets = []struct {
	target
	constructor func() datasource.DataSource
}{
	{target{"database", "Databases"}, provider.NewDatabaseDataSource},
	{target{"databases", "Databases"}, provider.NewDatabasesDataSource},
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

// importIDs maps resource names to their terraform import identifier format.
var importIDs = map[string]string{
	"database":                  "my-workspace/app-database",
	"database_compute_instance": "my-workspace/app-database/inst-abc123",
	"database_credential":       "my-workspace/app-database/cred-abc123",
	"developer_invite":          "my-workspace/inv-abc123",
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

	required := make([]string, 0, len(names))
	computed := make([]string, 0, len(names))
	for _, name := range names {
		if attrs[name].IsRequired() {
			required = append(required, name)
		} else {
			computed = append(computed, name)
		}
	}

	b.WriteString("\n## Argument Reference\n")
	if len(required) == 0 {
		b.WriteString("\nThis object has no required arguments.\n")
	}
	for _, name := range required {
		writeAttribute(&b, attrs[name], name, "  ")
	}

	b.WriteString("\n## Attribute Reference\n")
	if len(computed) == 0 {
		b.WriteString("\nThis object has no computed attributes.\n")
	}
	for _, name := range computed {
		writeAttribute(&b, attrs[name], name, "  ")
	}

	if importID != "" {
		fmt.Fprintf(&b, "\n## Import\n\nImport an existing object with:\n\n```sh\nterraform import %s.example %s\n```\n", typeName, importID)
	}
	return b.String()
}

func writeAttribute(b *strings.Builder, a attribute, name, indent string) {
	mode := "Computed"
	switch {
	case a.IsRequired():
		mode = "Required"
	case a.IsOptional() && a.IsComputed():
		mode = "Optional, Computed"
	case a.IsOptional():
		mode = "Optional"
	}
	if a.IsSensitive() {
		mode += ", Sensitive"
	}
	description := strings.TrimSpace(a.GetMarkdownDescription())
	fmt.Fprintf(b, "%s- `%s` - (%s) %s\n", indent, name, mode, description)
	if nested := nestedAttributes(a); nested != nil {
		nestedNames := make([]string, 0, len(nested))
		for n := range nested {
			nestedNames = append(nestedNames, n)
		}
		sort.Strings(nestedNames)
		for _, n := range nestedNames {
			writeAttribute(b, nested[n], n, indent+"  ")
		}
	}
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
