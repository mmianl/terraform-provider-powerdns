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
	viewsBackendHint = "\n\nViews and networks are only implemented by the LMDB backend. " +
		"The generic SQL backends have no tables for them, so a read returns an empty " +
		"list while a write fails like this. Check the launch= setting on the server."

	recursorAPIDirHint = "\n\nThe recursor only accepts writes when api-config-dir is set " +
		"(webservice.api_dir in the YAML settings). It is unset by default, which makes " +
		"the whole recursor API read-only."
)

// viewsHint explains the backend requirement behind a rejected view or network
// write. Only 422 is annotated: a 404 or 401 means something else entirely.
func viewsHint(statusCode int) string {
	if statusCode == http.StatusUnprocessableEntity {
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
