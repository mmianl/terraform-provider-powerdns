package powerdns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// requestOptions describes a single API call. Every PowerDNS endpoint differs
// only in which statuses it treats as success, whether a 404 is a real absence
// rather than a failure, and which backend hint explains a rejection, so those
// are the only knobs here.
type requestOptions struct {
	// method is the HTTP method; defaults to GET when empty.
	method string
	// endpoint is the API path, already escaped by pathEscape.
	endpoint string
	// body, when non-nil, is marshalled as the JSON request body.
	body any
	// out, when non-nil, receives the decoded JSON response body.
	out any
	// okCodes lists the statuses that count as success. Defaults to 200.
	okCodes []int
	// notFoundErr, when set, is returned instead of a generic error for a 404.
	notFoundErr error
	// describe names the operation for error messages, e.g. `zone "example.com."`.
	describe string
	// hint optionally expands on a server rejection; see backend_hints.go.
	hint func(statusCode int, serverMessage string) string
	// logFields are attached to the response-close warning, if any.
	logFields map[string]any
}

// pathEscape escapes a single path segment for use in an API endpoint. Zone
// names, metadata kinds and view names all reach the API as user input and can
// legitimately contain characters that would otherwise change the request path.
func pathEscape(segment string) string {
	return url.PathEscape(segment)
}

// do performs one API request end to end: it marshals the body, sends the
// request, closes the response, maps the status to an error or to ErrNotFound,
// and decodes the payload into opts.out.
//
// It returns the status code alongside the error so the few callers that treat
// a non-success status as data (rather than as a failure) can inspect it.
func (client *BaseClient) do(ctx context.Context, opts requestOptions) (int, error) {
	method := opts.method
	if method == "" {
		method = http.MethodGet
	}

	var body []byte
	if opts.body != nil {
		var err error
		if body, err = json.Marshal(opts.body); err != nil {
			return 0, fmt.Errorf("error encoding request for %s: %w", opts.describe, err)
		}
	}

	req, err := client.newRequest(ctx, method, opts.endpoint, body)
	if err != nil {
		return 0, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fields := map[string]any{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
			}
			for k, v := range opts.logFields {
				fields[k] = v
			}
			tflog.Warn(ctx, "Error closing response body", fields)
		}
	}()

	okCodes := opts.okCodes
	if len(okCodes) == 0 {
		okCodes = []int{http.StatusOK}
	}

	if !containsStatus(okCodes, resp.StatusCode) {
		if opts.notFoundErr != nil && resp.StatusCode == http.StatusNotFound {
			return resp.StatusCode, opts.notFoundErr
		}
		return resp.StatusCode, client.statusError(resp, opts)
	}

	if opts.out != nil {
		if err := json.NewDecoder(resp.Body).Decode(opts.out); err != nil {
			return resp.StatusCode, fmt.Errorf("error decoding response for %s: %w", opts.describe, err)
		}
	}

	return resp.StatusCode, nil
}

// statusError turns an unexpected status into an error, preferring the server's
// own message and appending a backend hint when one applies.
func (client *BaseClient) statusError(resp *http.Response, opts requestOptions) error {
	var errorResp errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
		// The body was not the JSON error envelope the API documents, so the
		// status and the decode failure are all there is to report.
		return fmt.Errorf("error with %s: %s: failed to decode error response: %w",
			opts.describe, resp.Status, err)
	}

	hint := ""
	if opts.hint != nil {
		hint = opts.hint(resp.StatusCode, errorResp.ErrorMsg)
	}

	return fmt.Errorf("error with %s, reason: %q%s", opts.describe, errorResp.ErrorMsg, hint)
}

func containsStatus(codes []int, code int) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

// newRequest creates a new request against the API with necessary headers.
func (client *BaseClient) newRequest(ctx context.Context, method string, endpoint string, body []byte) (*http.Request, error) {
	apiVersion, err := client.apiVersion(ctx)
	if err != nil {
		return nil, err
	}

	var urlStr string
	if apiVersion > 0 {
		urlStr = client.ServerURL + "/api/v" + strconv.Itoa(apiVersion) + endpoint
	} else {
		urlStr = client.ServerURL + endpoint
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("error during parsing request URL: %s", err)
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("error during creation of request: %s", err)
	}

	req.Header.Add("X-API-Key", client.APIKey)
	req.Header.Add("Accept", "application/json")

	if method != http.MethodGet {
		req.Header.Add("Content-Type", "application/json")
	}

	return req, nil
}

// apiVersion returns the server's API version, detecting it at most once even
// when Terraform walks the resource graph in parallel over a shared client.
// A version already set on the client (APIVersion >= 0) is taken as given and
// suppresses detection entirely.
func (client *BaseClient) apiVersion(ctx context.Context) (int, error) {
	client.apiVersionOnce.Do(func() {
		if client.APIVersion >= 0 {
			return
		}

		version, err := client.detectAPIVersion(ctx)
		if err != nil {
			client.apiVersionErr = err
			return
		}
		client.APIVersion = version
	})

	return client.APIVersion, client.apiVersionErr
}

// detectAPIVersion detects the API version in use on the server.
// Uses int to represent the API version: 0 is the legacy AKA version 3.4 API.
// Any other integer correlates with the same API version.
func (client *BaseClient) detectAPIVersion(ctx context.Context) (int, error) {
	u, err := url.Parse(client.ServerURL + "/api/v1/servers")
	if err != nil {
		return -1, fmt.Errorf("error while trying to detect the API version, request URL: %s", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return -1, fmt.Errorf("error during creation of request: %s", err)
	}

	req.Header.Add("X-API-Key", client.APIKey)
	req.Header.Add("Accept", "application/json")

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return -1, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]any{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
			})
		}
	}()

	if resp.StatusCode == http.StatusOK {
		return 1, nil
	}
	return 0, nil
}
