// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWorkspaceComputeUsageSchema(t *testing.T) {
	sch := dataSourceSchemaOf(t, NewWorkspaceComputeUsageDataSource())
	for _, name := range []string{"workspace_slug", "start", "end", "id", "periods"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("nile_workspace_compute_usage: missing attribute %q", name)
		}
	}
}

func TestReadComputeUsageSortsMaps(t *testing.T) {
	client := testAPIServer(t, `[{
		"totalVCPUHours":3,
		"usageByDatabase":{
			"zebra":{"totalVCPUHours":2,"usageByInstance":{"b":{"totalVCPUHours":1},"a":{"totalVCPUHours":1}}},
			"alpha":{"totalVCPUHours":1}
		}
	}]`)
	data := &workspaceComputeUsageDataSourceModel{WorkspaceSlug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readWorkspaceComputeUsage(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if len(data.Periods) != 1 || len(data.Periods[0].Databases) != 2 {
		t.Fatalf("periods = %+v", data.Periods)
	}
	if data.Periods[0].Databases[0].Name.ValueString() != "alpha" ||
		data.Periods[0].Databases[1].Name.ValueString() != "zebra" {
		t.Errorf("database order = %s, %s", data.Periods[0].Databases[0].Name, data.Periods[0].Databases[1].Name)
	}
	instances := data.Periods[0].Databases[1].Instances
	if len(instances) != 2 || instances[0].Name.ValueString() != "a" || instances[1].Name.ValueString() != "b" {
		t.Errorf("instance order = %+v", instances)
	}
	if data.Periods[0].TotalVCPUHours.ValueFloat64() != 3 {
		t.Errorf("total_vcpu_hours = %v", data.Periods[0].TotalVCPUHours)
	}
}
