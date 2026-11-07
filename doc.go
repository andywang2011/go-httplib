// Package httplib provides an HTTP client with configurable outbound request logging.
//
// Design constraints:
//   - ConfigDefault: N/A — clients are built via functional options (WithTimeout, etc.).
//   - Requires go-logging SetupFromConfig before outbound logs are emitted.
//   - Body logging is opt-in via WithMaxBodyLogBytes or NewDebugTransport.
//
// By default only metadata is logged (method, URL, headers, status) without buffering bodies.
// Use WithMaxBodyLogBytes or NewDebugTransport to enable truncated body logging for debugging.
//
// Requires go-logging to be initialized (SetupFromConfig) before requests are logged.
// When logging is not set up, requests still succeed but log lines are silently dropped.
package httplib
