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
	typeName string
	schema   func(context.Context) schema.Schema
	read     func(context.Context, *nileapi.Client, *M, *datasource.ReadResponse)
	client   *nileapi.Client
}

// newReadOnlyDataSource wires a model type to its schema and read function.
func newReadOnlyDataSource[M any](
	typeName string,
	schemaFn func(context.Context) schema.Schema,
	read func(context.Context, *nileapi.Client, *M, *datasource.ReadResponse),
) datasource.DataSource {
	return &readOnlyDataSource[M]{typeName: typeName, schema: schemaFn, read: read}
}

var (
	_ datasource.DataSource              = (*readOnlyDataSource[struct{}])(nil)
	_ datasource.DataSourceWithConfigure = (*readOnlyDataSource[struct{}])(nil)
)

func (d *readOnlyDataSource[M]) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = d.typeName
}

func (d *readOnlyDataSource[M]) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = d.schema(ctx)
}

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
