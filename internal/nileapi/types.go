// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"encoding/json"
	"strconv"
	"strings"
)

// rawUnmarshaler is implemented by types that keep the full server payload in
// a Raw field so attributes the API adds later stay accessible to callers.
type rawUnmarshaler interface {
	setRaw(json.RawMessage)
}

// decodeInto unmarshals data into v and, when the target type preserves raw
// payloads, stores a copy of data there. It is the shared plumbing behind the
// UnmarshalJSON methods of the API model types.
func decodeInto(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return err
	}
	if r, ok := v.(rawUnmarshaler); ok {
		r.setRaw(append(json.RawMessage(nil), data...))
	}
	return nil
}

// flexInt64 accepts JSON numbers as well as numeric strings. Some Nile
// responses encode the same identifier as number in one schema and as string
// in another.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if s == "" || s == "null" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*f = flexInt64(v)
	return nil
}

// Workspace is a Nile workspace.
type Workspace struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Slug             string          `json:"slug"`
	StripeCustomerID string          `json:"stripe_customer_id"`
	Created          string          `json:"created"`
	Raw              json.RawMessage `json:"-"`
}

func (w *Workspace) UnmarshalJSON(data []byte) error { return decodeInto(data, (*workspaceAlias)(w)) }

type workspaceAlias Workspace

func (w *workspaceAlias) setRaw(raw json.RawMessage) { w.Raw = raw }

// DatabaseParent is the parent (primary) database of a read replica.
type DatabaseParent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Database is a Nile database. The API uses two closely related shapes:
// GetDatabaseResponse (string region, apiHost/dbHost) and Database (regionId),
// so this struct accepts both.
type Database struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Workspace  *Workspace      `json:"workspace"`
	Status     string          `json:"status"`
	Region     string          `json:"region"`
	RegionID   flexInt64       `json:"regionId"`
	Expandable bool            `json:"expandable"`
	Created    string          `json:"created"`
	Deleted    string          `json:"deleted"`
	APIHost    string          `json:"apiHost"`
	DBHost     string          `json:"dbHost"`
	Parent     *DatabaseParent `json:"parent"`
	Raw        json.RawMessage `json:"-"`
}

func (d *Database) UnmarshalJSON(data []byte) error { return decodeInto(data, (*databaseAlias)(d)) }

type databaseAlias Database

func (d *databaseAlias) setRaw(raw json.RawMessage) { d.Raw = raw }

// Ready reports whether the database has finished provisioning and can be
// used. A missing status is not treated as ready because it may represent a
// partial asynchronous response.
func (d Database) Ready() bool { return databaseReady(d.Status) }

// ComputeInstanceType is a dedicated compute instance sizing option.
type ComputeInstanceType struct {
	ID          string  `json:"id"`
	ComputeSize string  `json:"computeSize"`
	Memory      string  `json:"memory"`
	HourlyCost  float64 `json:"hourlyCost"`
}

// ComputeInstance is one dedicated compute instance attached to a database.
// The API is evolving, so the full server payload is preserved in Raw while
// the most useful fields are promoted to typed values (best effort).
//
// The promoted fields map to the API response as follows:
//
//	ID        <- instanceId
//	Name      <- instanceName
//	Status    <- status
//	Size      <- instanceType.computeSize
//	Region    <- region
//	CreatedAt <- created
//
// Attributes the API adds or renames later remain accessible via Raw.
type ComputeInstance struct {
	ID         string  `json:"instanceId"`
	Name       string  `json:"instanceName"`
	Status     string  `json:"status"`
	Size       string  `json:"-"`
	Region     string  `json:"region"`
	CreatedAt  string  `json:"created"`
	Updated    string  `json:"updated"`
	Deleted    string  `json:"deleted"`
	Memory     string  `json:"-"`
	HourlyCost float64 `json:"-"`

	InstanceType        *ComputeInstanceType `json:"instanceType"`
	DesiredInstanceType *ComputeInstanceType `json:"desiredInstanceType"`
	Workspace           *Workspace           `json:"workspace"`
	Database            *Database            `json:"database"`
	Raw                 json.RawMessage      `json:"-"`
}

// Ready reports whether the compute instance is usable. A missing status is
// not treated as ready because it may represent a partial asynchronous
// response.
func (c ComputeInstance) Ready() bool {
	return c.Status == "READY"
}

// Credential is a database credential. Password is only populated by
// CreateCredential and RotateCredential; the API never returns it again.
type Credential struct {
	ID       string          `json:"id"`
	Database *Database       `json:"database"`
	Tenant   string          `json:"tenant"`
	Internal bool            `json:"internal"`
	Password string          `json:"password"`
	Created  string          `json:"created"`
	Raw      json.RawMessage `json:"-"`
}

func (c *Credential) UnmarshalJSON(data []byte) error { return decodeInto(data, (*credentialAlias)(c)) }

type credentialAlias Credential

func (c *credentialAlias) setRaw(raw json.RawMessage) { c.Raw = raw }

// Developer is a Nile developer (a user of the control plane).
type Developer struct {
	ID         string          `json:"id"`
	Email      string          `json:"email"`
	Kind       string          `json:"kind"`
	Workspaces []Workspace     `json:"workspaces"`
	Databases  []Database      `json:"databases"`
	Raw        json.RawMessage `json:"-"`
}

func (d *Developer) UnmarshalJSON(data []byte) error { return decodeInto(data, (*developerAlias)(d)) }

type developerAlias Developer

func (d *developerAlias) setRaw(raw json.RawMessage) { d.Raw = raw }

// DeveloperInvite is an invitation for an email address to join a workspace.
// Code is only populated when the invite was created with programmatic=true.
type DeveloperInvite struct {
	ID                string          `json:"id"`
	Sender            *Developer      `json:"sender"`
	Email             string          `json:"email"`
	Workspace         *Workspace      `json:"workspace"`
	VerificationState string          `json:"verificationState"`
	Created           string          `json:"created"`
	Updated           string          `json:"updated"`
	Code              string          `json:"code"`
	Raw               json.RawMessage `json:"-"`
}

func (i *DeveloperInvite) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*developerInviteAlias)(i))
}

type developerInviteAlias DeveloperInvite

func (i *developerInviteAlias) setRaw(raw json.RawMessage) { i.Raw = raw }

// DatabaseUptimePoint is one uptime sample.
type DatabaseUptimePoint struct {
	Timestamp        string  `json:"timestamp"`
	UptimePercentage float64 `json:"uptimePercentage"`
	UptimeSeconds    int64   `json:"uptimeSeconds"`
	ObservedSeconds  int64   `json:"observedSeconds"`
}

// DatabaseUptimeSummary aggregates the uptime over the requested window.
type DatabaseUptimeSummary struct {
	UptimePercentage float64 `json:"uptimePercentage"`
	UptimeSeconds    int64   `json:"uptimeSeconds"`
	ObservedSeconds  int64   `json:"observedSeconds"`
}

// DatabaseUptimeResponse is the response of the uptime insights endpoint.
type DatabaseUptimeResponse struct {
	Source      string                 `json:"source"`
	Scope       string                 `json:"scope"`
	Granularity string                 `json:"granularity"`
	Calculation string                 `json:"calculation"`
	Summary     *DatabaseUptimeSummary `json:"summary"`
	Points      []DatabaseUptimePoint  `json:"points"`
	Raw         json.RawMessage        `json:"-"`
}

func (r *DatabaseUptimeResponse) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*databaseUptimeResponseAlias)(r))
}

type databaseUptimeResponseAlias DatabaseUptimeResponse

func (r *databaseUptimeResponseAlias) setRaw(raw json.RawMessage) { r.Raw = raw }

// DatabaseErrorPoint is one bucketed error count.
type DatabaseErrorPoint struct {
	Timestamp  string `json:"timestamp"`
	Source     string `json:"source"`
	ErrorCount int64  `json:"errorCount"`
}

// DatabaseErrorsResponse is the response of the error insights endpoint.
type DatabaseErrorsResponse struct {
	Granularity string               `json:"granularity"`
	Points      []DatabaseErrorPoint `json:"points"`
	Raw         json.RawMessage      `json:"-"`
}

func (r *DatabaseErrorsResponse) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*databaseErrorsResponseAlias)(r))
}

type databaseErrorsResponseAlias DatabaseErrorsResponse

func (r *databaseErrorsResponseAlias) setRaw(raw json.RawMessage) { r.Raw = raw }

// DatabaseQueryPerformancePoint is one query performance sample.
type DatabaseQueryPerformancePoint struct {
	Timestamp             string  `json:"timestamp"`
	ProxyQueriesPerSecond float64 `json:"proxyQueriesPerSecond"`
	ThothQueriesPerSecond float64 `json:"thothQueriesPerSecond"`
	ThothP99LatencyMs     float64 `json:"thothP99LatencyMs"`
	ThothCPUMilliseconds  float64 `json:"thothCpuMilliseconds"`
}

// DatabaseQueryPerformanceResponse is the response of the query performance
// insights endpoint.
type DatabaseQueryPerformanceResponse struct {
	Source      string                          `json:"source"`
	Granularity string                          `json:"granularity"`
	Points      []DatabaseQueryPerformancePoint `json:"points"`
	Raw         json.RawMessage                 `json:"-"`
}

func (r *DatabaseQueryPerformanceResponse) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*databaseQueryPerformanceResponseAlias)(r))
}

type databaseQueryPerformanceResponseAlias DatabaseQueryPerformanceResponse

func (r *databaseQueryPerformanceResponseAlias) setRaw(raw json.RawMessage) { r.Raw = raw }

// InstancePeriodComputeUsage is the compute usage of one instance or span.
type InstancePeriodComputeUsage struct {
	TotalVCPUHours float64                      `json:"totalVCPUHours"`
	Size           string                       `json:"size"`
	Start          string                       `json:"start"`
	End            string                       `json:"end"`
	Spans          []InstancePeriodComputeUsage `json:"spans"`
}

// DatabaseComputeUsage is the compute usage of one database.
type DatabaseComputeUsage struct {
	UsageByInstance map[string]InstancePeriodComputeUsage `json:"usageByInstance"`
	TotalVCPUHours  float64                               `json:"totalVCPUHours"`
	Start           string                                `json:"start"`
	End             string                                `json:"end"`
}

// TimeSeriesPoint is one point of a usage chart.
type TimeSeriesPoint struct {
	X int64   `json:"x"`
	Y float64 `json:"y"`
}

// UsageChartData is the chart data of a compute usage response.
type UsageChartData struct {
	Points      []TimeSeriesPoint `json:"points"`
	MaxCPUCount int64             `json:"maxCPUCount"`
}

// WorkspaceComputeUsage is one compute usage period of a workspace.
type WorkspaceComputeUsage struct {
	UsageByDatabase map[string]DatabaseComputeUsage `json:"usageByDatabase"`
	ChartData       *UsageChartData                 `json:"chartData"`
	TotalVCPUHours  float64                         `json:"totalVCPUHours"`
	Start           string                          `json:"start"`
	End             string                          `json:"end"`
	Raw             json.RawMessage                 `json:"-"`
}

func (u *WorkspaceComputeUsage) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*workspaceComputeUsageAlias)(u))
}

type workspaceComputeUsageAlias WorkspaceComputeUsage

func (u *workspaceComputeUsageAlias) setRaw(raw json.RawMessage) { u.Raw = raw }

// WorkspaceBillingCustomer is the Stripe customer linked to a workspace.
type WorkspaceBillingCustomer struct {
	Workspace            string          `json:"workspace"`
	StripeCustomerID     string          `json:"stripeCustomerId"`
	DefaultPaymentMethod string          `json:"defaultPaymentMethod"`
	Raw                  json.RawMessage `json:"-"`
}

func (c *WorkspaceBillingCustomer) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*workspaceBillingCustomerAlias)(c))
}

type workspaceBillingCustomerAlias WorkspaceBillingCustomer

func (c *workspaceBillingCustomerAlias) setRaw(raw json.RawMessage) { c.Raw = raw }

// WorkspaceBillingReadiness describes whether a workspace can be billed.
type WorkspaceBillingReadiness struct {
	Workspace            string          `json:"workspace"`
	WorkspaceID          string          `json:"workspaceId"`
	StripeCustomerID     string          `json:"stripeCustomerId"`
	DefaultPaymentMethod string          `json:"defaultPaymentMethodId"`
	Status               string          `json:"status"`
	CheckedAt            string          `json:"checkedAt"`
	LastError            string          `json:"lastError"`
	Source               string          `json:"source"`
	Detail               string          `json:"detail"`
	Raw                  json.RawMessage `json:"-"`
}

func (r *WorkspaceBillingReadiness) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*workspaceBillingReadinessAlias)(r))
}

type workspaceBillingReadinessAlias WorkspaceBillingReadiness

func (r *workspaceBillingReadinessAlias) setRaw(raw json.RawMessage) { r.Raw = raw }

// WorkspaceSubscription is one subscription row of a workspace.
type WorkspaceSubscription struct {
	Workspace            string          `json:"workspace"`
	Level                string          `json:"level"`
	ValidFrom            string          `json:"validFrom"`
	ValidTo              string          `json:"validTo"`
	SubscriptionID       string          `json:"subscriptionId"`
	DefaultPaymentMethod string          `json:"defaultPaymentMethod"`
	Raw                  json.RawMessage `json:"-"`
}

func (s *WorkspaceSubscription) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*workspaceSubscriptionAlias)(s))
}

type workspaceSubscriptionAlias WorkspaceSubscription

func (s *workspaceSubscriptionAlias) setRaw(raw json.RawMessage) { s.Raw = raw }

// WorkspaceMonthlyTotals are the rated monthly totals of a workspace.
type WorkspaceMonthlyTotals struct {
	YM     string             `json:"ym"`
	Totals map[string]float64 `json:"totals"`
	Raw    json.RawMessage    `json:"-"`
}

func (t *WorkspaceMonthlyTotals) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*workspaceMonthlyTotalsAlias)(t))
}

type workspaceMonthlyTotalsAlias WorkspaceMonthlyTotals

func (t *workspaceMonthlyTotalsAlias) setRaw(raw json.RawMessage) { t.Raw = raw }

// ProvisionedDatabase is the best-effort decoded response of the
// unauthenticated provision endpoint. The API does not document the response
// schema, so only the claim code is promoted and the full payload stays
// available in Raw.
type ProvisionedDatabase struct {
	ClaimCode    string            `json:"claimCode"`
	DatabaseID   string            `json:"databaseId"`
	DatabaseName string            `json:"databaseName"`
	APIHost      string            `json:"apiHost"`
	DBHost       string            `json:"dbHost"`
	CredentialID string            `json:"credentialId"`
	Password     string            `json:"password"`
	Sharded      bool              `json:"sharded"`
	Env          map[string]string `json:"env"`
	Raw          json.RawMessage   `json:"-"`
}

func (p *ProvisionedDatabase) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*provisionedDatabaseAlias)(p))
}

type provisionedDatabaseAlias ProvisionedDatabase

func (p *provisionedDatabaseAlias) setRaw(raw json.RawMessage) { p.Raw = raw }

// TokenResponse is the response of the OAuth2 token endpoint.
type TokenResponse struct {
	AccessToken  string          `json:"access_token"`
	TokenType    string          `json:"token_type"`
	ExpiresIn    flexInt64       `json:"expires_in"`
	RefreshToken string          `json:"refresh_token"`
	Scope        string          `json:"scope"`
	Raw          json.RawMessage `json:"-"`
}

func (t *TokenResponse) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*tokenResponseAlias)(t))
}

type tokenResponseAlias TokenResponse

func (t *tokenResponseAlias) setRaw(raw json.RawMessage) { t.Raw = raw }

// derefString returns the value behind a *string, or "" for nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
