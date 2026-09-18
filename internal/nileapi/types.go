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

// UnmarshalJSON accepts a JSON number or a numeric string ("42" and 42)
// and stores the parsed value; empty and null inputs leave the field unset.
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
	// ID is the workspace identifier.
	ID string `json:"id"`
	// Name is the workspace display name.
	Name string `json:"name"`
	// Slug is the URL-safe workspace identifier used in API paths.
	Slug string `json:"slug"`
	// StripeCustomerID is the Stripe customer linked to the workspace, if any.
	StripeCustomerID string `json:"stripe_customer_id"`
	// Created is the creation timestamp as returned by the API.
	Created string `json:"created"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the workspace and preserves the full payload in Raw
// via decodeInto; workspaceAlias prevents infinite recursion.
func (w *Workspace) UnmarshalJSON(data []byte) error { return decodeInto(data, (*workspaceAlias)(w)) }

// workspaceAlias mirrors Workspace without its UnmarshalJSON method so that
// decoding into the alias cannot recurse.
type workspaceAlias Workspace

// setRaw records the raw server payload (rawUnmarshaler).
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
	// ID is the database identifier.
	ID string `json:"id"`
	// Name is the database name.
	Name string `json:"name"`
	// Workspace is the embedded workspace summary, when the response includes it.
	Workspace *Workspace `json:"workspace"`
	// Status is the database lifecycle status (for example READY).
	Status string `json:"status"`
	// Region is the region identifier of the database (GetDatabase shape).
	Region string `json:"region"`
	// RegionID is the numeric region identifier (list shape); flexInt64 accepts
	// a number or a string.
	RegionID flexInt64 `json:"regionId"`
	// Expandable reports whether the API marks the database as expandable.
	Expandable bool `json:"expandable"`
	// Created is the creation timestamp of the database.
	Created string `json:"created"`
	// Deleted is the deletion timestamp, empty while the database exists.
	Deleted string `json:"deleted"`
	// APIHost is the API endpoint host of the database (GetDatabase shape).
	APIHost string `json:"apiHost"`
	// DBHost is the SQL connection host of the database (GetDatabase shape).
	DBHost string `json:"dbHost"`
	// Parent is the primary database when this database is a read replica, nil
	// otherwise.
	Parent *DatabaseParent `json:"parent"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes either documented API shape and preserves the full
// payload in Raw via decodeInto; databaseAlias prevents infinite recursion.
func (d *Database) UnmarshalJSON(data []byte) error { return decodeInto(data, (*databaseAlias)(d)) }

// databaseAlias mirrors Database without its UnmarshalJSON method so that
// decoding into the alias cannot recurse.
type databaseAlias Database

// setRaw records the raw server payload (rawUnmarshaler).
func (d *databaseAlias) setRaw(raw json.RawMessage) { d.Raw = raw }

// Ready reports whether the database has finished provisioning and can be
// used. A missing status is not treated as ready because it may represent a
// partial asynchronous response.
func (d Database) Ready() bool { return databaseReady(d.Status) }

// ComputeInstanceType is a dedicated compute instance sizing option.
type ComputeInstanceType struct {
	// ID is the instance type identifier.
	ID string `json:"id"`
	// ComputeSize is the size identifier of the instance type.
	ComputeSize string `json:"computeSize"`
	// Memory is the memory available to the size, as returned by the API.
	Memory string `json:"memory"`
	// HourlyCost is the estimated hourly cost of the size.
	HourlyCost float64 `json:"hourlyCost"`
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
	// ID is the instance identifier (instanceId in the API response).
	ID string `json:"instanceId"`
	// Name is the instance name (instanceName in the API response).
	Name string `json:"instanceName"`
	// Status is the instance lifecycle status (for example READY).
	Status string `json:"status"`
	// Size is promoted from InstanceType.ComputeSize when that field decoded.
	Size string `json:"-"`
	// Region is the region the instance runs in.
	Region string `json:"region"`
	// CreatedAt is the creation timestamp (created in the API response).
	CreatedAt string `json:"created"`
	// Updated is the last-update timestamp as returned by the API.
	Updated string `json:"updated"`
	// Deleted is the deletion timestamp, empty while the instance exists.
	Deleted string `json:"deleted"`
	// Memory is promoted from InstanceType.Memory when that field decoded.
	Memory string `json:"-"`
	// HourlyCost is promoted from InstanceType.HourlyCost when that field decoded.
	HourlyCost float64 `json:"-"`

	// InstanceType is the current sizing of the instance.
	InstanceType *ComputeInstanceType `json:"instanceType"`
	// DesiredInstanceType is the sizing the instance is moving to during a
	// resize, when one is set.
	DesiredInstanceType *ComputeInstanceType `json:"desiredInstanceType"`
	// Workspace is the embedded workspace, when the response includes it.
	Workspace *Workspace `json:"workspace"`
	// Database is the embedded database, when the response includes it.
	Database *Database `json:"database"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
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
	// ID is the credential identifier.
	ID string `json:"id"`
	// Database is the database the credential belongs to.
	Database *Database `json:"database"`
	// Tenant is the tenant the credential authenticates as.
	Tenant string `json:"tenant"`
	// Internal reports whether the credential is an internal system credential.
	Internal bool `json:"internal"`
	// Password is the secret password; only create and rotate responses carry it.
	Password string `json:"password"`
	// Created is the creation timestamp of the credential.
	Created string `json:"created"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the credential and preserves the full payload in Raw
// via decodeInto; credentialAlias prevents infinite recursion.
func (c *Credential) UnmarshalJSON(data []byte) error { return decodeInto(data, (*credentialAlias)(c)) }

// credentialAlias mirrors Credential without its UnmarshalJSON method so
// that decoding into the alias cannot recurse.
type credentialAlias Credential

// setRaw records the raw server payload (rawUnmarshaler).
func (c *credentialAlias) setRaw(raw json.RawMessage) { c.Raw = raw }

// Developer is a Nile developer (a user of the control plane).
type Developer struct {
	// ID is the developer identifier.
	ID string `json:"id"`
	// Email is the developer's login email address.
	Email string `json:"email"`
	// Kind is the developer account kind as reported by the API.
	Kind string `json:"kind"`
	// Workspaces are the workspaces associated with the developer.
	Workspaces []Workspace `json:"workspaces"`
	// Databases are the databases associated with the developer.
	Databases []Database `json:"databases"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the developer and preserves the full payload in Raw
// via decodeInto; developerAlias prevents infinite recursion.
func (d *Developer) UnmarshalJSON(data []byte) error { return decodeInto(data, (*developerAlias)(d)) }

// developerAlias mirrors Developer without its UnmarshalJSON method so that
// decoding into the alias cannot recurse.
type developerAlias Developer

// setRaw records the raw server payload (rawUnmarshaler).
func (d *developerAlias) setRaw(raw json.RawMessage) { d.Raw = raw }

// DeveloperInvite is an invitation for an email address to join a workspace.
// Code is only populated when the invite was created with programmatic=true.
type DeveloperInvite struct {
	// ID is the invite identifier.
	ID string `json:"id"`
	// Sender is the developer who created the invite.
	Sender *Developer `json:"sender"`
	// Email is the invited email address.
	Email string `json:"email"`
	// Workspace is the workspace the invite grants access to.
	Workspace *Workspace `json:"workspace"`
	// VerificationState is the invite lifecycle state (EMAIL_PENDING,
	// EMAIL_SENT, VERIFIED or EXPIRED).
	VerificationState string `json:"verificationState"`
	// Created is the creation timestamp of the invite.
	Created string `json:"created"`
	// Updated is the last-update timestamp of the invite.
	Updated string `json:"updated"`
	// Code is the invite code; it is only set for programmatic invites.
	Code string `json:"code"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the invite and preserves the full payload in Raw via
// decodeInto; developerInviteAlias prevents infinite recursion.
func (i *DeveloperInvite) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*developerInviteAlias)(i))
}

// developerInviteAlias mirrors DeveloperInvite without its UnmarshalJSON
// method so that decoding into the alias cannot recurse.
type developerInviteAlias DeveloperInvite

// setRaw records the raw server payload (rawUnmarshaler).
func (i *developerInviteAlias) setRaw(raw json.RawMessage) { i.Raw = raw }

// DatabaseUptimePoint is one uptime sample.
type DatabaseUptimePoint struct {
	// Timestamp is the start of the sampling bucket.
	Timestamp string `json:"timestamp"`
	// UptimePercentage is the share of the bucket the database was up.
	UptimePercentage float64 `json:"uptimePercentage"`
	// UptimeSeconds is how long the database was up within the bucket.
	UptimeSeconds int64 `json:"uptimeSeconds"`
	// ObservedSeconds is how long the database was observed within the bucket.
	ObservedSeconds int64 `json:"observedSeconds"`
}

// DatabaseUptimeSummary aggregates the uptime over the requested window.
type DatabaseUptimeSummary struct {
	// UptimePercentage is the share of the window the database was up.
	UptimePercentage float64 `json:"uptimePercentage"`
	// UptimeSeconds is how long the database was up within the window.
	UptimeSeconds int64 `json:"uptimeSeconds"`
	// ObservedSeconds is how long the database was observed within the window.
	ObservedSeconds int64 `json:"observedSeconds"`
}

// DatabaseUptimeResponse is the response of the uptime insights endpoint.
type DatabaseUptimeResponse struct {
	// Source identifies the origin of the uptime data.
	Source string `json:"source"`
	// Scope is the scope the series was computed for.
	Scope string `json:"scope"`
	// Granularity is the bucket size of the points.
	Granularity string `json:"granularity"`
	// Calculation is how the uptime was aggregated.
	Calculation string `json:"calculation"`
	// Summary aggregates the uptime over the requested window.
	Summary *DatabaseUptimeSummary `json:"summary"`
	// Points are the per-bucket uptime samples.
	Points []DatabaseUptimePoint `json:"points"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the uptime response and preserves the full payload
// in Raw via decodeInto; databaseUptimeResponseAlias prevents infinite
// recursion.
func (r *DatabaseUptimeResponse) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*databaseUptimeResponseAlias)(r))
}

// databaseUptimeResponseAlias mirrors DatabaseUptimeResponse without its
// UnmarshalJSON method so that decoding into the alias cannot recurse.
type databaseUptimeResponseAlias DatabaseUptimeResponse

// setRaw records the raw server payload (rawUnmarshaler).
func (r *databaseUptimeResponseAlias) setRaw(raw json.RawMessage) { r.Raw = raw }

// DatabaseErrorPoint is one bucketed error count.
type DatabaseErrorPoint struct {
	// Timestamp is the start of the sampling bucket.
	Timestamp string `json:"timestamp"`
	// Source identifies the origin of the error data.
	Source string `json:"source"`
	// ErrorCount is the number of errors observed in the bucket.
	ErrorCount int64 `json:"errorCount"`
}

// DatabaseErrorsResponse is the response of the error insights endpoint.
type DatabaseErrorsResponse struct {
	// Granularity is the bucket size of the points.
	Granularity string `json:"granularity"`
	// Points are the per-bucket error counts.
	Points []DatabaseErrorPoint `json:"points"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the error response and preserves the full payload in
// Raw via decodeInto; databaseErrorsResponseAlias prevents infinite
// recursion.
func (r *DatabaseErrorsResponse) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*databaseErrorsResponseAlias)(r))
}

// databaseErrorsResponseAlias mirrors DatabaseErrorsResponse without its
// UnmarshalJSON method so that decoding into the alias cannot recurse.
type databaseErrorsResponseAlias DatabaseErrorsResponse

// setRaw records the raw server payload (rawUnmarshaler).
func (r *databaseErrorsResponseAlias) setRaw(raw json.RawMessage) { r.Raw = raw }

// DatabaseQueryPerformancePoint is one query performance sample.
type DatabaseQueryPerformancePoint struct {
	// Timestamp is the start of the sampling bucket.
	Timestamp string `json:"timestamp"`
	// ProxyQueriesPerSecond is the queries-per-second served by the proxy.
	ProxyQueriesPerSecond float64 `json:"proxyQueriesPerSecond"`
	// ThothQueriesPerSecond is the queries-per-second served by Thoth.
	ThothQueriesPerSecond float64 `json:"thothQueriesPerSecond"`
	// ThothP99LatencyMs is the p99 query latency in milliseconds.
	ThothP99LatencyMs float64 `json:"thothP99LatencyMs"`
	// ThothCPUMilliseconds is the CPU time consumed by queries in milliseconds.
	ThothCPUMilliseconds float64 `json:"thothCpuMilliseconds"`
}

// DatabaseQueryPerformanceResponse is the response of the query performance
// insights endpoint.
type DatabaseQueryPerformanceResponse struct {
	// Source identifies the origin of the query performance data.
	Source string `json:"source"`
	// Granularity is the bucket size of the points.
	Granularity string `json:"granularity"`
	// Points are the per-bucket query performance samples.
	Points []DatabaseQueryPerformancePoint `json:"points"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the query performance response and preserves the
// full payload in Raw via decodeInto; databaseQueryPerformanceResponseAlias
// prevents infinite recursion.
func (r *DatabaseQueryPerformanceResponse) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*databaseQueryPerformanceResponseAlias)(r))
}

// databaseQueryPerformanceResponseAlias mirrors
// DatabaseQueryPerformanceResponse without its UnmarshalJSON method so that
// decoding into the alias cannot recurse.
type databaseQueryPerformanceResponseAlias DatabaseQueryPerformanceResponse

// setRaw records the raw server payload (rawUnmarshaler).
func (r *databaseQueryPerformanceResponseAlias) setRaw(raw json.RawMessage) { r.Raw = raw }

// InstancePeriodComputeUsage is the compute usage of one instance or span.
type InstancePeriodComputeUsage struct {
	// TotalVCPUHours is the total vCPU-hours consumed in the period.
	TotalVCPUHours float64 `json:"totalVCPUHours"`
	// Size is the instance size the usage belongs to.
	Size string `json:"size"`
	// Start is the start of the period.
	Start string `json:"start"`
	// End is the end of the period.
	End string `json:"end"`
	// Spans are the sub-intervals the usage is split into.
	Spans []InstancePeriodComputeUsage `json:"spans"`
}

// DatabaseComputeUsage is the compute usage of one database.
type DatabaseComputeUsage struct {
	// UsageByInstance maps instance IDs to their compute usage.
	UsageByInstance map[string]InstancePeriodComputeUsage `json:"usageByInstance"`
	// TotalVCPUHours is the total vCPU-hours consumed by the database.
	TotalVCPUHours float64 `json:"totalVCPUHours"`
	// Start is the start of the period.
	Start string `json:"start"`
	// End is the end of the period.
	End string `json:"end"`
}

// TimeSeriesPoint is one point of a usage chart.
type TimeSeriesPoint struct {
	// X is the x value of the point as provided by the API.
	X int64 `json:"x"`
	// Y is the y value of the point as provided by the API.
	Y float64 `json:"y"`
}

// UsageChartData is the chart data of a compute usage response.
type UsageChartData struct {
	// Points are the chart's time series points.
	Points []TimeSeriesPoint `json:"points"`
	// MaxCPUCount is the maximum CPU count in the series.
	MaxCPUCount int64 `json:"maxCPUCount"`
}

// WorkspaceComputeUsage is one compute usage period of a workspace.
type WorkspaceComputeUsage struct {
	// UsageByDatabase maps database names to their compute usage.
	UsageByDatabase map[string]DatabaseComputeUsage `json:"usageByDatabase"`
	// ChartData holds the pre-rendered chart series of the period.
	ChartData *UsageChartData `json:"chartData"`
	// TotalVCPUHours is the total vCPU-hours consumed by the workspace.
	TotalVCPUHours float64 `json:"totalVCPUHours"`
	// Start is the start of the period.
	Start string `json:"start"`
	// End is the end of the period.
	End string `json:"end"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the compute usage and preserves the full payload in
// Raw via decodeInto; workspaceComputeUsageAlias prevents infinite recursion.
func (u *WorkspaceComputeUsage) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*workspaceComputeUsageAlias)(u))
}

// workspaceComputeUsageAlias mirrors WorkspaceComputeUsage without its
// UnmarshalJSON method so that decoding into the alias cannot recurse.
type workspaceComputeUsageAlias WorkspaceComputeUsage

// setRaw records the raw server payload (rawUnmarshaler).
func (u *workspaceComputeUsageAlias) setRaw(raw json.RawMessage) { u.Raw = raw }

// WorkspaceBillingCustomer is the Stripe customer linked to a workspace.
type WorkspaceBillingCustomer struct {
	// Workspace is the workspace slug.
	Workspace string `json:"workspace"`
	// StripeCustomerID is the Stripe customer linked to the workspace.
	StripeCustomerID string `json:"stripeCustomerId"`
	// DefaultPaymentMethod is the customer's default payment method ID, if any.
	DefaultPaymentMethod string `json:"defaultPaymentMethod"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the billing customer and preserves the full payload
// in Raw via decodeInto; workspaceBillingCustomerAlias prevents infinite
// recursion.
func (c *WorkspaceBillingCustomer) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*workspaceBillingCustomerAlias)(c))
}

// workspaceBillingCustomerAlias mirrors WorkspaceBillingCustomer without its
// UnmarshalJSON method so that decoding into the alias cannot recurse.
type workspaceBillingCustomerAlias WorkspaceBillingCustomer

// setRaw records the raw server payload (rawUnmarshaler).
func (c *workspaceBillingCustomerAlias) setRaw(raw json.RawMessage) { c.Raw = raw }

// WorkspaceBillingReadiness describes whether a workspace can be billed.
type WorkspaceBillingReadiness struct {
	// Workspace is the workspace slug.
	Workspace string `json:"workspace"`
	// WorkspaceID is the workspace identifier.
	WorkspaceID string `json:"workspaceId"`
	// StripeCustomerID is the Stripe customer linked to the workspace, if any.
	StripeCustomerID string `json:"stripeCustomerId"`
	// DefaultPaymentMethod is the customer's default payment method ID, if any.
	DefaultPaymentMethod string `json:"defaultPaymentMethodId"`
	// Status is the readiness status, e.g. "ready" or "missing_customer".
	Status string `json:"status"`
	// CheckedAt is the timestamp of the readiness check.
	CheckedAt string `json:"checkedAt"`
	// LastError is the last error observed while resolving billing state, if any.
	LastError string `json:"lastError"`
	// Source is the origin of the readiness result.
	Source string `json:"source"`
	// Detail is human-readable detail about the readiness result.
	Detail string `json:"detail"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the billing readiness and preserves the full payload
// in Raw via decodeInto; workspaceBillingReadinessAlias prevents infinite
// recursion.
func (r *WorkspaceBillingReadiness) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*workspaceBillingReadinessAlias)(r))
}

// workspaceBillingReadinessAlias mirrors WorkspaceBillingReadiness without
// its UnmarshalJSON method so that decoding into the alias cannot recurse.
type workspaceBillingReadinessAlias WorkspaceBillingReadiness

// setRaw records the raw server payload (rawUnmarshaler).
func (r *workspaceBillingReadinessAlias) setRaw(raw json.RawMessage) { r.Raw = raw }

// WorkspaceSubscription is one subscription row of a workspace.
type WorkspaceSubscription struct {
	// Workspace is the workspace slug.
	Workspace string `json:"workspace"`
	// Level is the subscription level.
	Level string `json:"level"`
	// ValidFrom is the start of the validity window.
	ValidFrom string `json:"validFrom"`
	// ValidTo is the end of the validity window.
	ValidTo string `json:"validTo"`
	// SubscriptionID is the subscription identifier.
	SubscriptionID string `json:"subscriptionId"`
	// DefaultPaymentMethod is the customer's default payment method ID, if any.
	DefaultPaymentMethod string `json:"defaultPaymentMethod"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the subscription and preserves the full payload in
// Raw via decodeInto; workspaceSubscriptionAlias prevents infinite recursion.
func (s *WorkspaceSubscription) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*workspaceSubscriptionAlias)(s))
}

// workspaceSubscriptionAlias mirrors WorkspaceSubscription without its
// UnmarshalJSON method so that decoding into the alias cannot recurse.
type workspaceSubscriptionAlias WorkspaceSubscription

// setRaw records the raw server payload (rawUnmarshaler).
func (s *workspaceSubscriptionAlias) setRaw(raw json.RawMessage) { s.Raw = raw }

// WorkspaceMonthlyTotals are the rated monthly totals of a workspace.
type WorkspaceMonthlyTotals struct {
	// YM is the month in YYYY-MM form.
	YM string `json:"ym"`
	// Totals maps billing components to rated totals.
	Totals map[string]float64 `json:"totals"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the monthly totals and preserves the full payload in
// Raw via decodeInto; workspaceMonthlyTotalsAlias prevents infinite
// recursion.
func (t *WorkspaceMonthlyTotals) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*workspaceMonthlyTotalsAlias)(t))
}

// workspaceMonthlyTotalsAlias mirrors WorkspaceMonthlyTotals without its
// UnmarshalJSON method so that decoding into the alias cannot recurse.
type workspaceMonthlyTotalsAlias WorkspaceMonthlyTotals

// setRaw records the raw server payload (rawUnmarshaler).
func (t *workspaceMonthlyTotalsAlias) setRaw(raw json.RawMessage) { t.Raw = raw }

// ProvisionedDatabase is the best-effort decoded response of the
// unauthenticated provision endpoint. The API does not document the response
// schema, so only the claim code is promoted and the full payload stays
// available in Raw.
type ProvisionedDatabase struct {
	// ClaimCode is the code used to claim the provisioned database.
	ClaimCode string `json:"claimCode"`
	// DatabaseID is the identifier of the provisioned database.
	DatabaseID string `json:"databaseId"`
	// DatabaseName is the name of the provisioned database.
	DatabaseName string `json:"databaseName"`
	// APIHost is the API endpoint host of the database.
	APIHost string `json:"apiHost"`
	// DBHost is the SQL connection host of the database.
	DBHost string `json:"dbHost"`
	// CredentialID is the identifier of the created credential.
	CredentialID string `json:"credentialId"`
	// Password is the initial credential password.
	Password string `json:"password"`
	// Sharded reports whether the database was created sharded.
	Sharded bool `json:"sharded"`
	// Env holds environment variables returned by the provisioner.
	Env map[string]string `json:"env"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the provisioning response and preserves the full
// payload in Raw via decodeInto; provisionedDatabaseAlias prevents infinite
// recursion.
func (p *ProvisionedDatabase) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*provisionedDatabaseAlias)(p))
}

// provisionedDatabaseAlias mirrors ProvisionedDatabase without its
// UnmarshalJSON method so that decoding into the alias cannot recurse.
type provisionedDatabaseAlias ProvisionedDatabase

// setRaw records the raw server payload (rawUnmarshaler).
func (p *provisionedDatabaseAlias) setRaw(raw json.RawMessage) { p.Raw = raw }

// TokenResponse is the response of the OAuth2 token endpoint.
type TokenResponse struct {
	// AccessToken is the bearer token for API calls.
	AccessToken string `json:"access_token"`
	// TokenType is the OAuth2 token type (for example Bearer).
	TokenType string `json:"token_type"`
	// ExpiresIn is the token lifetime in seconds; flexInt64 accepts a number
	// or a numeric string.
	ExpiresIn flexInt64 `json:"expires_in"`
	// RefreshToken is the token used to obtain a new access token.
	RefreshToken string `json:"refresh_token"`
	// Scope is the granted scope of the token.
	Scope string `json:"scope"`
	// Raw preserves the full server payload for forward compatibility.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the token response and preserves the full payload in
// Raw via decodeInto; tokenResponseAlias prevents infinite recursion.
func (t *TokenResponse) UnmarshalJSON(data []byte) error {
	return decodeInto(data, (*tokenResponseAlias)(t))
}

// tokenResponseAlias mirrors TokenResponse without its UnmarshalJSON method
// so that decoding into the alias cannot recurse.
type tokenResponseAlias TokenResponse

// setRaw records the raw server payload (rawUnmarshaler).
func (t *tokenResponseAlias) setRaw(raw json.RawMessage) { t.Raw = raw }

// derefString returns the value behind a *string, or "" for nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
