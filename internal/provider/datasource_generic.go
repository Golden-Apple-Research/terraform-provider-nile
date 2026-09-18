// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// readOnlyDataSource implements the shared plumbing of every Nile data source:
// metadata, schema, provider client wiring, and state bookkeeping. A concrete
// data source only supplies its schema and a read function that populates the
// model in place.
type readOnlyDataSource[M any] struct {
	// typeName is the Terraform data source type name (e.g. "nile_database").
	typeName string
	// schema builds the data source schema.
	schema func(context.Context) schema.Schema
	// read fetches the data from the API and populates the model in place.
	read func(context.Context, *nileapi.Client, *M, *datasource.ReadResponse)
	// client is the Nile API client injected by Configure.
	client *nileapi.Client
}

// newReadOnlyDataSource wires a model type to its schema and read function.
func newReadOnlyDataSource[M any](
	typeName string,
	schemaFn func(context.Context) schema.Schema,
	read func(context.Context, *nileapi.Client, *M, *datasource.ReadResponse),
) datasource.DataSource {
	return &readOnlyDataSource[M]{typeName: typeName, schema: schemaFn, read: read}
}

// Compile-time assertions that readOnlyDataSource implements the required
// data source interfaces.
var (
	_ datasource.DataSource              = (*readOnlyDataSource[struct{}])(nil)
	_ datasource.DataSourceWithConfigure = (*readOnlyDataSource[struct{}])(nil)
)

// Metadata sets the data source type name.
func (d *readOnlyDataSource[M]) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = d.typeName
}

// Schema returns the schema supplied by the concrete data source.
func (d *readOnlyDataSource[M]) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = d.schema(ctx)
}

// Configure stores the *nileapi.Client handed out by the provider.
func (d *readOnlyDataSource[M]) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*nileapi.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("Expected *nileapi.Client, got: %T.", req.ProviderData),
		)
		return
	}
	d.client = client
}

// Read loads the config, invokes the concrete read function, and writes the
// populated model into state.
func (d *readOnlyDataSource[M]) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config M
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}
	d.read(ctx, d.client, &config, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// dataSourceID builds the conventional data source ID from its arguments.
func dataSourceID(parts ...string) string {
	return joinResourceID(parts...)
}
