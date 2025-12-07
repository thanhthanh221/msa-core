package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GenerateCurlCommand generates a cURL command string from request details.
func GenerateCurlCommand(method, url string, headers map[string]string, body interface{}) (string, error) {
	var cmd []string
	cmd = append(cmd, "curl", "-X", method, fmt.Sprintf("'%s'", url))

	for key, value := range headers {
		cmd = append(cmd, "-H", fmt.Sprintf("'%s: %s'", key, value))
	}

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("failed to marshal body: %w", err)
		}

		// For GET request, append body as query parameters
		if method == http.MethodGet {
			// This is a simplified version. A full implementation would need to handle URL encoding
			// and merging with existing query parameters.
			if len(bodyBytes) > 0 {
				// Assuming body is a JSON object that can be converted to query params
				// This part might need to be more sophisticated depending on the actual body structure.
			}
		} else {
			// For other methods like POST, PUT, DELETE
			cmd = append(cmd, "-d", fmt.Sprintf("'%s'", string(bodyBytes)))
		}
	}

	return strings.Join(cmd, " "), nil
}

// GenerateCurlCommandFromRequest generates a cURL command string from an http.Request.
func GenerateCurlCommandFromRequest(req *http.Request) (string, error) {
	var cmd []string

	// Method and URL
	cmd = append(cmd, "curl", "-X", req.Method, fmt.Sprintf("'%s'", req.URL.String()))

	// Headers
	for key, values := range req.Header {
		for _, value := range values {
			cmd = append(cmd, "-H", fmt.Sprintf("'%s: %s'", key, value))
		}
	}

	// Body
	if req.Body != nil {
		bodyBytes := new(bytes.Buffer)
		_, err := bodyBytes.ReadFrom(req.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read request body: %w", err)
		}
		// Restore the body so it can be read again
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes.Bytes()))

		if bodyBytes.Len() > 0 {
			cmd = append(cmd, "-d", fmt.Sprintf("'%s'", bodyBytes.String()))
		}
	}

	return strings.Join(cmd, " "), nil
}
