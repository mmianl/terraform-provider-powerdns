package powerdns

import (
	"net/http"
	"strings"
)

// The PowerDNS API answers an unsupported operation with 422 and a message
// describing the immediate failure rather than the requirement behind it. Two
// of those requirements come up often enough to be worth naming, because the
// server's own wording does not point at the fix.

const (
	viewsBackendHint = "\n\nPowerDNS authoritative views and networks require the LMDB backend, " +
		"views=yes, and a non-zero zone-cache-refresh-interval. Check these settings " +
		"on the server."

	recursorAPIDirHint = "\n\nThe recursor only accepts writes when api-config-dir is set " +
		"(webservice.api_dir in the YAML settings). It is unset by default, which makes " +
		"the whole recursor API read-only."
)

// viewsHint explains the backend requirement only when PowerDNS reports the
// failure emitted after a view-capable backend operation returns false. Other
// 422 responses describe semantic errors and must retain their original meaning.
func viewsHint(statusCode int, serverMessage string) string {
	if statusCode != http.StatusUnprocessableEntity {
		return ""
	}

	switch {
	case strings.HasPrefix(serverMessage, "Failed to add ") && strings.Contains(serverMessage, " to view "):
		return viewsBackendHint
	case strings.HasPrefix(serverMessage, "Failed to remove ") && strings.Contains(serverMessage, " from view "):
		return viewsBackendHint
	case strings.HasPrefix(serverMessage, "Failed to setup view ") && strings.Contains(serverMessage, " for network "):
		return viewsBackendHint
	}
	return ""
}

// recursorHint explains the api-config-dir requirement when the recursor is
// complaining about it, which it does by name.
func recursorHint(serverMessage string) string {
	if strings.Contains(serverMessage, "api-config-dir") {
		return recursorAPIDirHint
	}
	return ""
}

// recursorMessageHint adapts recursorHint to the hint signature used by
// requestOptions; the recursor names the missing setting regardless of status.
func recursorMessageHint(_ int, serverMessage string) string {
	return recursorHint(serverMessage)
}
