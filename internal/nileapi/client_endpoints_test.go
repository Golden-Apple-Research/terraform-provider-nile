// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type requestCapture struct {
	method      string
	path        string
	query       string
	contentType string
	body        string
}

// captureServer serves a fixed response and records the request it received.
func captureServer(t *testing.T, status int, response string) (*httptest.Server, *requestCapture) {
	t.Helper()
	cap := &requestCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cap.method = r.Method
		cap.path = r.URL.EscapedPath()
		cap.query = r.URL.RawQuery
		cap.contentType = r.Header.Get("Content-Type")
		cap.body = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	return srv, cap
}

func testClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := NewClient(srv.URL, "tok-1")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.retrySleep = instantSleep
	return c
}

func TestListDatabases(t *testing.T) {
	srv, cap := captureServer(t, 200, `[{"id":"db-1","name":"app","status":"READY","region":"AWS_US_WEST_2","expandable":true,"apiHost":"api.example","workspace":{"slug":"ws"}}]`)
	c := testClient(t, srv)

	got, err := c.ListDatabases(t.Context(), "ws 1")
	if err != nil {
		t.Fatalf("ListDatabases: %v", err)
	}
	if cap.method != http.MethodGet || cap.path != "/workspaces/ws%201/databases" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if len(got) != 1 || got[0].Name != "app" || got[0].Region != "AWS_US_WEST_2" ||
		!got[0].Expandable || got[0].APIHost != "api.example" || got[0].Workspace.Slug != "ws" {
		t.Errorf("databases = %+v", got)
	}
	if len(got[0].Raw) == 0 {
		t.Error("Raw payload not preserved")
	}
}

func TestCreateDatabaseRequestShape(t *testing.T) {
	srv, cap := captureServer(t, 201, `{"id":"db-9","name":"app","status":"PENDING","region":"AWS_EU_CENTRAL_1"}`)
	c := testClient(t, srv)

	db, err := c.CreateDatabase(t.Context(), "ws", CreateDatabaseRequest{DatabaseName: "app", Region: "AWS_EU_CENTRAL_1"})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/workspaces/ws/databases" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.contentType != "application/json" {
		t.Errorf("content type = %q", cap.contentType)
	}
	var body map[string]string
	if err := json.Unmarshal([]byte(cap.body), &body); err != nil {
		t.Fatalf("request body %q: %v", cap.body, err)
	}
	if body["databaseName"] != "app" || body["region"] != "AWS_EU_CENTRAL_1" {
		t.Errorf("body = %v", body)
	}
	if db.ID != "db-9" || db.Ready() {
		t.Errorf("database = %+v, Ready() = %v", db, db.Ready())
	}
}

func TestGetDatabaseNotFound(t *testing.T) {
	srv, _ := captureServer(t, 404, `{"errorCode":"entity_not_found","message":"no such database","statusCode":404}`)
	c := testClient(t, srv)

	_, err := c.GetDatabase(t.Context(), "ws", "missing")
	if !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
	if ErrorCode(err) != "entity_not_found" {
		t.Errorf("ErrorCode = %q", ErrorCode(err))
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "no such database" {
		t.Errorf("APIError = %+v", apiErr)
	}
}

func TestRenameDatabase(t *testing.T) {
	srv, cap := captureServer(t, 200, `{"id":"db-1","name":"new-name","status":"READY","region":"AWS_US_WEST_2"}`)
	c := testClient(t, srv)

	db, err := c.RenameDatabase(t.Context(), "ws", "old-name", "new-name")
	if err != nil {
		t.Fatalf("RenameDatabase: %v", err)
	}
	if cap.method != http.MethodPut || cap.path != "/workspaces/ws/databases/old-name" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.body != `{"name":"new-name"}` {
		t.Errorf("body = %q", cap.body)
	}
	if db.Name != "new-name" {
		t.Errorf("database = %+v", db)
	}
}

func TestDeleteDatabase(t *testing.T) {
	srv, cap := captureServer(t, 200, `{"id":"db-1","name":"app","status":"READY"}`)
	c := testClient(t, srv)

	if _, err := c.DeleteDatabase(t.Context(), "ws", "app"); err != nil {
		t.Fatalf("DeleteDatabase: %v", err)
	}
	if cap.method != http.MethodDelete || cap.path != "/workspaces/ws/databases/app" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
}

func TestProvisionAndClaimDatabase(t *testing.T) {
	srv, cap := captureServer(t, 200, `{"claimCode":"code-1","database":{"id":"db-1"}}`)
	c := testClient(t, srv)

	prov, err := c.ProvisionDatabase(t.Context(), "AWS_US_WEST_2")
	if err != nil {
		t.Fatalf("ProvisionDatabase: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/databases/provision" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if prov.ClaimCode != "code-1" {
		t.Errorf("claim code = %q", prov.ClaimCode)
	}

	srv2, cap2 := captureServer(t, 200, `{"id":"db-1","name":"app","status":"READY"}`)
	c2 := testClient(t, srv2)
	if _, err := c2.ClaimDatabase(t.Context(), "ws", "code-1"); err != nil {
		t.Fatalf("ClaimDatabase: %v", err)
	}
	if cap2.path != "/workspaces/ws/databases/claim" {
		t.Errorf("path = %s", cap2.path)
	}
	if cap2.body != `{"claimCode":"code-1"}` {
		t.Errorf("body = %q", cap2.body)
	}
}

func TestCreateComputeInstanceSingleObject(t *testing.T) {
	srv, cap := captureServer(t, 202, `{"instanceId":"inst-1","instanceName":"primary","instanceType":{"computeSize":"large","memory":"8GB","hourlyCost":0.42},"status":"PENDING"}`)
	c := testClient(t, srv)

	instance, err := c.CreateComputeInstance(t.Context(), "ws", "db", "primary", "large")
	if err != nil {
		t.Fatalf("CreateComputeInstance: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/workspaces/ws/databases/db/compute" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.body != `{"instanceName":"primary","instanceSize":"large"}` {
		t.Errorf("body = %q", cap.body)
	}
	if instance.ID != "inst-1" || instance.Size != "large" || instance.Memory != "8GB" || instance.HourlyCost != 0.42 {
		t.Errorf("instance = %+v", instance)
	}
}

func TestDescribeComputeInstanceArray(t *testing.T) {
	srv, cap := captureServer(t, 200, `[{"instanceId":"inst-1","status":"READY"},{"instanceId":"inst-2","status":"READY"}]`)
	c := testClient(t, srv)

	instance, err := c.DescribeComputeInstance(t.Context(), "ws", "db", "inst-2")
	if err != nil {
		t.Fatalf("DescribeComputeInstance: %v", err)
	}
	if cap.path != "/workspaces/ws/databases/db/compute/inst-2" {
		t.Errorf("path = %s", cap.path)
	}
	if instance.ID != "inst-2" {
		t.Errorf("instance = %+v, want the instance matching the requested ID", instance)
	}
}

func TestUpdateComputeInstanceOmitsEmptyFields(t *testing.T) {
	srv, cap := captureServer(t, 202, `{"id":"db-1"}`)
	c := testClient(t, srv)

	err := c.UpdateComputeInstance(t.Context(), "ws", "db", "inst-1", UpdateComputeInstanceRequest{InstanceSize: "xlarge"})
	if err != nil {
		t.Fatalf("UpdateComputeInstance: %v", err)
	}
	if cap.method != http.MethodPut || cap.path != "/workspaces/ws/databases/db/compute/inst-1" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.body != `{"instanceSize":"xlarge"}` {
		t.Errorf("body = %q", cap.body)
	}
}

func TestDeleteComputeInstance(t *testing.T) {
	srv, cap := captureServer(t, 200, `{"id":"db-1"}`)
	c := testClient(t, srv)

	if err := c.DeleteComputeInstance(t.Context(), "ws", "db", "inst-1"); err != nil {
		t.Fatalf("DeleteComputeInstance: %v", err)
	}
	if cap.method != http.MethodDelete || cap.path != "/workspaces/ws/databases/db/compute/inst-1" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
}

func TestListCredentialsQuery(t *testing.T) {
	srv, cap := captureServer(t, 200, `[{"id":"cred-1","tenant":"acme","internal":true,"database":{"apiHost":"api.example","dbHost":"db.example"}}]`)
	c := testClient(t, srv)
	internal := true

	got, err := c.ListCredentials(t.Context(), "ws", "db", "acme", &internal)
	if err != nil {
		t.Fatalf("ListCredentials: %v", err)
	}
	if cap.path != "/workspaces/ws/databases/db/credentials" {
		t.Errorf("path = %s", cap.path)
	}
	q, _ := url.ParseQuery(cap.query)
	if q.Get("tenantId") != "acme" || q.Get("internal") != "true" {
		t.Errorf("query = %q", cap.query)
	}
	if len(got) != 1 || got[0].Tenant != "acme" || !got[0].Internal || got[0].Database.APIHost != "api.example" {
		t.Errorf("credentials = %+v", got)
	}
}

func TestCreateAndRotateCredential(t *testing.T) {
	srv, cap := captureServer(t, 200, `{"id":"cred-1","password":"s3cret","tenant":"acme","internal":false,"database":{"apiHost":"api.example"},"created":"2025-06-01T00:00:00Z"}`)
	c := testClient(t, srv)

	cred, err := c.CreateCredential(t.Context(), "ws", "db", "", nil)
	if err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/workspaces/ws/databases/db/credentials" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.query != "" {
		t.Errorf("query = %q, want empty", cap.query)
	}
	if cred.Password != "s3cret" || cred.ID != "cred-1" {
		t.Errorf("credential = %+v", cred)
	}

	srv2, cap2 := captureServer(t, 200, `{"id":"cred-2","password":"new"}`)
	c2 := testClient(t, srv2)
	if _, err := c2.RotateCredential(t.Context(), "ws", "db", "acme", nil, RotateCredentialRequest{DelayOldSecretsExpirationHours: 24, Reason: "scheduled"}); err != nil {
		t.Fatalf("RotateCredential: %v", err)
	}
	if cap2.path != "/workspaces/ws/databases/db/credentials/rotate" {
		t.Errorf("path = %s", cap2.path)
	}
	if cap2.body != `{"delayOldSecretsExpirationHours":24,"reason":"scheduled"}` {
		t.Errorf("body = %q", cap2.body)
	}
}

func TestDeleteCredentialRejectsEmptyID(t *testing.T) {
	c, err := NewClient("http://127.0.0.1:1", "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.DeleteCredential(t.Context(), "ws", "db", ""); err == nil {
		t.Fatal("expected an error for an empty credential ID")
	}
}

func TestWorkspaceEndpoints(t *testing.T) {
	srv, cap := captureServer(t, 200, `[{"id":"ws-1","name":"Acme","slug":"acme","created":"2025-01-01T00:00:00Z"}]`)
	c := testClient(t, srv)

	workspaces, err := c.ListWorkspaces(t.Context())
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if cap.path != "/workspaces" || len(workspaces) != 1 || workspaces[0].Slug != "acme" {
		t.Errorf("path=%s workspaces=%+v", cap.path, workspaces)
	}

	ws, err := c.GetWorkspace(t.Context(), "acme")
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if cap.path != "/workspaces/acme" || ws.ID != "ws-1" {
		t.Errorf("path=%s workspace=%+v", cap.path, ws)
	}
}

func TestGetWorkspaceEmptyListIsNotFound(t *testing.T) {
	srv, _ := captureServer(t, 200, `[]`)
	c := testClient(t, srv)

	_, err := c.GetWorkspace(t.Context(), "missing")
	if !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestCreateWorkspace(t *testing.T) {
	srv, cap := captureServer(t, 201, `{"id":"ws-1","name":"Acme","slug":"acme"}`)
	c := testClient(t, srv)

	ws, err := c.CreateWorkspace(t.Context(), "Acme")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/workspaces" || cap.body != `{"name":"Acme"}` {
		t.Errorf("request = %s %s %s", cap.method, cap.path, cap.body)
	}
	if ws.Slug != "acme" {
		t.Errorf("workspace = %+v", ws)
	}
}

func TestListRegionsAndComputeTypes(t *testing.T) {
	srv, cap := captureServer(t, 200, `["AWS_US_WEST_2","AZURE_EASTUS"]`)
	c := testClient(t, srv)

	regions, err := c.ListRegions(t.Context(), "ws")
	if err != nil {
		t.Fatalf("ListRegions: %v", err)
	}
	if cap.path != "/workspaces/ws/regions" || len(regions) != 2 || regions[1] != "AZURE_EASTUS" {
		t.Errorf("path=%s regions=%v", cap.path, regions)
	}

	srv2, cap2 := captureServer(t, 200, `[{"id":"t1","computeSize":"large","memory":"8GB","hourlyCost":0.42}]`)
	c2 := testClient(t, srv2)
	computeTypes, err := c2.ListComputeTypes(t.Context(), "ws")
	if err != nil {
		t.Fatalf("ListComputeTypes: %v", err)
	}
	if cap2.path != "/workspaces/ws/compute-types" || len(computeTypes) != 1 || computeTypes[0].ComputeSize != "large" {
		t.Errorf("path=%s types=%+v", cap2.path, computeTypes)
	}
}

func TestDeveloperEndpoints(t *testing.T) {
	srv, cap := captureServer(t, 200, `{"id":"dev-1","email":"a@example.com","kind":"HUMAN","workspaces":[{"slug":"acme"}]}`)
	c := testClient(t, srv)

	dev, err := c.GetDeveloper(t.Context())
	if err != nil {
		t.Fatalf("GetDeveloper: %v", err)
	}
	if cap.path != "/developers/me" || dev.Email != "a@example.com" || len(dev.Workspaces) != 1 {
		t.Errorf("path=%s developer=%+v", cap.path, dev)
	}

	srv2, cap2 := captureServer(t, 200, `[{"id":"inv-1","email":"b@example.com","verificationState":"EMAIL_SENT"}]`)
	c2 := testClient(t, srv2)
	invites, err := c2.ListDeveloperInvites(t.Context(), "acme", "EMAIL_SENT")
	if err != nil {
		t.Fatalf("ListDeveloperInvites: %v", err)
	}
	if cap2.path != "/workspaces/acme/invites" {
		t.Errorf("path = %s", cap2.path)
	}
	q, _ := url.ParseQuery(cap2.query)
	if q.Get("verificationState") != "EMAIL_SENT" {
		t.Errorf("query = %q", cap2.query)
	}
	if len(invites) != 1 || invites[0].ID != "inv-1" {
		t.Errorf("invites = %+v", invites)
	}
}

func TestCreateDeveloperInvite(t *testing.T) {
	srv, cap := captureServer(t, 201, `{"id":"inv-1","email":"b@example.com","verificationState":"EMAIL_PENDING","code":"invite-code"}`)
	c := testClient(t, srv)

	invite, err := c.CreateDeveloperInvite(t.Context(), "acme", "b@example.com", true)
	if err != nil {
		t.Fatalf("CreateDeveloperInvite: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/workspaces/acme/invites" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.body != `{"email":"b@example.com","programmatic":true}` {
		t.Errorf("body = %q", cap.body)
	}
	if invite.Code != "invite-code" || invite.ID != "inv-1" {
		t.Errorf("invite = %+v", invite)
	}
}

func TestRemoveDeveloperAndDeleteInvite(t *testing.T) {
	srv, cap := captureServer(t, 204, ``)
	c := testClient(t, srv)

	if err := c.RemoveWorkspaceDeveloper(t.Context(), "acme", "dev-2"); err != nil {
		t.Fatalf("RemoveWorkspaceDeveloper: %v", err)
	}
	if cap.path != "/workspaces/acme/developers/dev-2" || cap.method != http.MethodDelete {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}

	if err := c.DeleteDeveloperInvite(t.Context(), "acme", "inv-2"); err != nil {
		t.Fatalf("DeleteDeveloperInvite: %v", err)
	}
	if cap.path != "/workspaces/acme/invites/inv-2" {
		t.Errorf("path = %s", cap.path)
	}
}

func TestInsightsEndpoints(t *testing.T) {
	uptimeBody := `{"source":"probe","scope":"database","granularity":"1h","calculation":"time-weighted","summary":{"uptimePercentage":99.9,"uptimeSeconds":3599,"observedSeconds":3600},"points":[{"timestamp":"2025-06-01T00:00:00Z","uptimePercentage":99.5,"uptimeSeconds":3590,"observedSeconds":3600}]}`
	srv, cap := captureServer(t, 200, uptimeBody)
	c := testClient(t, srv)

	out, err := c.ListDatabaseUptime(t.Context(), "ws", "db", InsightsQuery{Start: "2025-06-01T00:00:00Z", Granularity: "1h"})
	if err != nil {
		t.Fatalf("ListDatabaseUptime: %v", err)
	}
	if cap.path != "/workspaces/ws/databases/db/insights/uptime" {
		t.Errorf("path = %s", cap.path)
	}
	q, _ := url.ParseQuery(cap.query)
	if q.Get("start") != "2025-06-01T00:00:00Z" || q.Get("granularity") != "1h" || q.Get("end") != "" {
		t.Errorf("query = %q", cap.query)
	}
	if out.Summary == nil || out.Summary.UptimePercentage != 99.9 || len(out.Points) != 1 || out.Points[0].UptimeSeconds != 3590 {
		t.Errorf("uptime = %+v", out)
	}
	if len(out.Raw) == 0 {
		t.Error("Raw payload not preserved")
	}

	srv2, cap2 := captureServer(t, 200, `{"granularity":"5m","points":[{"timestamp":"t","source":"proxy","errorCount":3}]}`)
	c2 := testClient(t, srv2)
	errs, err := c2.ListDatabaseErrors(t.Context(), "ws", "db", InsightsQuery{})
	if err != nil {
		t.Fatalf("ListDatabaseErrors: %v", err)
	}
	if cap2.path != "/workspaces/ws/databases/db/insights/errors" || len(errs.Points) != 1 || errs.Points[0].ErrorCount != 3 {
		t.Errorf("errors = %+v", errs)
	}

	srv3, cap3 := captureServer(t, 200, `{"source":"proxy","granularity":"1m","points":[{"timestamp":"t","proxyQueriesPerSecond":1.5,"thothQueriesPerSecond":1.2,"thothP99LatencyMs":10,"thothCpuMilliseconds":50}]}`)
	c3 := testClient(t, srv3)
	perf, err := c3.ListDatabaseQueryPerformance(t.Context(), "ws", "db", InsightsQuery{})
	if err != nil {
		t.Fatalf("ListDatabaseQueryPerformance: %v", err)
	}
	if cap3.path != "/workspaces/ws/databases/db/insights/query-performance" || len(perf.Points) != 1 || perf.Points[0].ThothP99LatencyMs != 10 {
		t.Errorf("performance = %+v", perf)
	}
}

func TestListWorkspaceComputeUsage(t *testing.T) {
	body := `[{"totalVCPUHours":12.5,"start":"2025-06-01T00:00:00Z","end":"2025-06-02T00:00:00Z","chartData":{"points":[{"x":1,"y":2}],"maxCPUCount":4},"usageByDatabase":{"app":{"totalVCPUHours":12.5,"usageByInstance":{"primary":{"totalVCPUHours":12.5,"size":"large"}}}}}]`
	srv, cap := captureServer(t, 200, body)
	c := testClient(t, srv)

	periods, err := c.ListWorkspaceComputeUsage(t.Context(), "ws", "2025-06-01T00:00:00Z", "")
	if err != nil {
		t.Fatalf("ListWorkspaceComputeUsage: %v", err)
	}
	if cap.path != "/workspaces/ws/metrics/compute" {
		t.Errorf("path = %s", cap.path)
	}
	q, _ := url.ParseQuery(cap.query)
	if q.Get("start") != "2025-06-01T00:00:00Z" {
		t.Errorf("query = %q", cap.query)
	}
	if len(periods) != 1 || periods[0].TotalVCPUHours != 12.5 ||
		periods[0].ChartData == nil || periods[0].ChartData.MaxCPUCount != 4 ||
		periods[0].UsageByDatabase["app"].UsageByInstance["primary"].Size != "large" {
		t.Errorf("periods = %+v", periods)
	}
}

func TestBillingEndpoints(t *testing.T) {
	srv, cap := captureServer(t, 200, `{"workspace":"acme","stripeCustomerId":"cus_1","defaultPaymentMethod":"pm_1"}`)
	c := testClient(t, srv)

	customer, err := c.EnsureBillingCustomer(t.Context(), "acme")
	if err != nil {
		t.Fatalf("EnsureBillingCustomer: %v", err)
	}
	if cap.method != http.MethodPut || cap.path != "/workspaces/acme/billing/customer" || customer.StripeCustomerID != "cus_1" {
		t.Errorf("request = %s %s customer=%+v", cap.method, cap.path, customer)
	}

	srv2, cap2 := captureServer(t, 200, `{"workspace":"acme","status":"ready","checkedAt":"2025-06-01T00:00:00Z"}`)
	c2 := testClient(t, srv2)
	readiness, err := c2.GetBillingReadiness(t.Context(), "acme")
	if err != nil {
		t.Fatalf("GetBillingReadiness: %v", err)
	}
	if cap2.path != "/workspaces/acme/billing/readiness" || readiness.Status != "ready" {
		t.Errorf("path=%s readiness=%+v", cap2.path, readiness)
	}

	srv3, cap3 := captureServer(t, 200, `{"ym":"2025-06","totals":{"compute":1.5,"storage":0.25}}`)
	c3 := testClient(t, srv3)
	totals, err := c3.GetMonthlyTotals(t.Context(), "acme", "2025-06")
	if err != nil {
		t.Fatalf("GetMonthlyTotals: %v", err)
	}
	if cap3.path != "/workspaces/acme/billing/2025-06/totals" || totals.Totals["compute"] != 1.5 {
		t.Errorf("path=%s totals=%+v", cap3.path, totals)
	}

	srv4, cap4 := captureServer(t, 200, `[{"workspace":"acme","level":"paid","subscriptionId":"sub_1"}]`)
	c4 := testClient(t, srv4)
	history, err := c4.ListSubscriptionHistory(t.Context(), "acme")
	if err != nil {
		t.Fatalf("ListSubscriptionHistory: %v", err)
	}
	if cap4.path != "/workspaces/acme/subscription/history" || len(history) != 1 || history[0].Level != "paid" {
		t.Errorf("path=%s history=%+v", cap4.path, history)
	}
}

func TestSubscriptionMutations(t *testing.T) {
	srv, cap := captureServer(t, 200, `{}`)
	c := testClient(t, srv)

	if err := c.StartSubscription(t.Context(), "acme", "paid"); err != nil {
		t.Fatalf("StartSubscription: %v", err)
	}
	if cap.method != http.MethodPost || cap.body != `{"level":"paid"}` {
		t.Errorf("start: %s %s", cap.method, cap.body)
	}
	if err := c.ChangeSubscription(t.Context(), "acme", "enterprise"); err != nil {
		t.Fatalf("ChangeSubscription: %v", err)
	}
	if cap.method != http.MethodPut || cap.body != `{"level":"enterprise"}` {
		t.Errorf("change: %s %s", cap.method, cap.body)
	}
	if err := c.CloseSubscription(t.Context(), "acme", "sub_9"); err != nil {
		t.Fatalf("CloseSubscription: %v", err)
	}
	if cap.method != http.MethodDelete || cap.path != "/workspaces/acme/subscription/sub_9" {
		t.Errorf("close: %s %s", cap.method, cap.path)
	}
}

func TestExchangeTokenFormEncoding(t *testing.T) {
	srv, cap := captureServer(t, 200, `{"access_token":"at","token_type":"bearer","expires_in":3600}`)
	c := testClient(t, srv)

	token, err := c.ExchangeToken(t.Context(), url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"rt"},
	})
	if err != nil {
		t.Fatalf("ExchangeToken: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/oauth2/token" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.contentType != "application/x-www-form-urlencoded" {
		t.Errorf("content type = %q", cap.contentType)
	}
	if cap.body != "grant_type=refresh_token&refresh_token=rt" {
		t.Errorf("body = %q", cap.body)
	}
	if token.AccessToken != "at" || int64(token.ExpiresIn) != 3600 {
		t.Errorf("token = %+v", token)
	}
}

func TestErrorBodyWithoutAPIErrorShape(t *testing.T) {
	srv, _ := captureServer(t, 500, `<html>gateway error</html>`)
	c := testClient(t, srv)
	c.MaxRetries = 0

	_, err := c.ListWorkspaces(t.Context())
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected plain HTTP error, got %v", err)
	}
}

func TestPostRetryPolicy(t *testing.T) {
	// A 503 on POST must not be replayed (the request may have been
	// processed); a 429 must be, including its body.
	var calls atomic.Int32
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		n := calls.Add(1)
		switch n {
		case 1:
			w.WriteHeader(http.StatusServiceUnavailable)
		case 2:
			w.WriteHeader(http.StatusTooManyRequests)
			w.Header().Set("Retry-After", "0")
		default:
			_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"READY"}`))
		}
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.CreateDatabase(t.Context(), "ws", CreateDatabaseRequest{DatabaseName: "app", Region: "AWS_US_WEST_2"})
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected the un-retried 503 to surface, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 (POST is not retried on 5xx)", calls.Load())
	}

	calls.Store(0)
	bodies = nil
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"READY"}`))
	}))
	defer srv2.Close()
	c2 := testClient(t, srv2)
	if _, err := c2.CreateDatabase(t.Context(), "ws", CreateDatabaseRequest{DatabaseName: "app", Region: "AWS_US_WEST_2"}); err != nil {
		t.Fatalf("CreateDatabase after 429: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2 (POST is retried on 429)", calls.Load())
	}
	if bodies[0] != bodies[1] {
		t.Errorf("retried body differs: %q vs %q", bodies[0], bodies[1])
	}
}

func TestWaitForDatabaseReady(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := "PENDING"
		if calls.Add(1) >= 3 {
			status = "READY"
		}
		_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"` + status + `"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	c.PollInterval = time.Millisecond
	db, err := c.WaitForDatabaseReady(t.Context(), "ws", "app")
	if err != nil {
		t.Fatalf("WaitForDatabaseReady: %v", err)
	}
	if db.Status != "READY" || calls.Load() != 3 {
		t.Errorf("status=%s calls=%d", db.Status, calls.Load())
	}
}

func TestWaitForDatabaseReadyNotFoundThenFound(t *testing.T) {
	// The database record may not be readable for a moment right after
	// creation; a 404 must be retried, not fail the wait.
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorCode":"entity_not_found"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"READY"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	c.PollInterval = time.Millisecond
	if _, err := c.WaitForDatabaseReady(t.Context(), "ws", "app"); err != nil {
		t.Fatalf("WaitForDatabaseReady: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestWaitForComputeInstanceReadyFails(t *testing.T) {
	srv, _ := captureServer(t, 200, `{"instanceId":"inst-1","status":"FAILED"}`)
	c := testClient(t, srv)

	_, err := c.WaitForComputeInstanceReady(t.Context(), "ws", "db", "inst-1")
	if err == nil || !strings.Contains(err.Error(), "FAILED") {
		t.Fatalf("expected terminal-status error, got %v", err)
	}
}

func TestWaitForComputeInstanceDeleted(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			_, _ = w.Write([]byte(`{"instanceId":"inst-1","status":"DELETING"}`))
		case 2:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorCode":"entity_not_found"}`))
		}
	}))
	defer srv.Close()

	c := testClient(t, srv)
	c.PollInterval = time.Millisecond
	if err := c.WaitForComputeInstanceDeleted(t.Context(), "ws", "db", "inst-1"); err != nil {
		t.Fatalf("WaitForComputeInstanceDeleted: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestWaitForDatabaseReadyHonorsContext(t *testing.T) {
	srv, _ := captureServer(t, 200, `{"id":"db-1","name":"app","status":"PENDING"}`)
	c := testClient(t, srv)
	c.PollInterval = time.Millisecond
	c.WaitTimeout = 20 * time.Millisecond

	_, err := c.WaitForDatabaseReady(t.Context(), "ws", "app")
	if err == nil || !strings.Contains(err.Error(), "waiting for database") {
		t.Fatalf("expected wait timeout, got %v", err)
	}
}

func TestMissingStatusIsNotReady(t *testing.T) {
	if (Database{}).Ready() {
		t.Fatal("database without a status must not be ready")
	}
	if (ComputeInstance{}).Ready() {
		t.Fatal("compute instance without a status must not be ready")
	}
}

func TestFlexInt64(t *testing.T) {
	var v struct {
		A flexInt64 `json:"a"`
		B flexInt64 `json:"b"`
	}
	if err := json.Unmarshal([]byte(`{"a":42,"b":"7"}`), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int64(v.A) != 42 || int64(v.B) != 7 {
		t.Errorf("v = %+v", v)
	}
}

func TestWithQuerySkipsEmptyValues(t *testing.T) {
	u, err := url.Parse("https://example.test/path")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	withQuery(u, "a", "", "b", "2", "c", "")
	if u.RawQuery != "b=2" {
		t.Errorf("query = %q", u.RawQuery)
	}
}
