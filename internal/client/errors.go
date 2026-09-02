// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// APIError is a non-2xx response from the Application API, carrying the parsed
// Laravel/Pelican {"errors":[...]} payload when present.
type APIError struct {
	StatusCode int
	Errors     []ErrorDetail
	Body       string
}

// ErrorDetail is one entry of the {"errors":[...]} payload.
type ErrorDetail struct {
	Code   string `json:"code"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func (e *APIError) Error() string {
	if len(e.Errors) > 0 {
		parts := make([]string, 0, len(e.Errors))
		for _, d := range e.Errors {
			switch {
			case d.Detail != "" && d.Code != "":
				parts = append(parts, fmt.Sprintf("%s: %s", d.Code, d.Detail))
			case d.Detail != "":
				parts = append(parts, d.Detail)
			case d.Code != "":
				parts = append(parts, d.Code)
			}
		}
		if len(parts) > 0 {
			return fmt.Sprintf("api error (HTTP %d): %s", e.StatusCode, strings.Join(parts, "; "))
		}
	}
	if e.Body != "" {
		return fmt.Sprintf("api error (HTTP %d): %s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("api error (HTTP %d)", e.StatusCode)
}

// NotFound reports whether the error is an HTTP 404.
func (e *APIError) NotFound() bool { return e.StatusCode == http.StatusNotFound }

func parseAPIError(status int, body []byte) *APIError {
	e := &APIError{StatusCode: status, Body: strings.TrimSpace(string(body))}
	var env struct {
		Errors []ErrorDetail `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err == nil && len(env.Errors) > 0 {
		e.Errors = env.Errors
	}
	return e
}

// IsNotFound reports whether err is (or wraps) an APIError with status 404. A
// resource Read uses it to self-heal — dropping the resource from state so tofu
// plans a fresh Create — rather than turning absence into a fatal diagnostic.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.NotFound()
	}
	return false
}
