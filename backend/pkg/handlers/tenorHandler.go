package handlers

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func readTenorKey() string {
	data, err := os.ReadFile("tenor.key")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func TenorProxyHandler(w http.ResponseWriter, r *http.Request) {
	apiKey := readTenorKey()
	clientKey := "social-network-app" // Use your app name

	if apiKey == "" {
		http.Error(w, "Tenor API key not configured", http.StatusInternalServerError)
		return
	}

	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		http.Error(w, "Missing endpoint", http.StatusBadRequest)
		return
	}

	allowedParams := map[string]map[string]bool{
		"search": {
			"q":             true,
			"limit":         true,
			"pos":           true,
			"contentfilter": true,
			"media_filter":  true,
			"ar_range":      true,
			"locale":        true,
		},
		"trending": {
			"limit":         true,
			"pos":           true,
			"contentfilter": true,
			"media_filter":  true,
			"ar_range":      true,
			"locale":        true,
		},
		"search_suggestions": {
			"q":      true,
			"limit":  true,
			"locale": true,
		},
		"categories": {
			"locale": true,
			"type":   true, // e.g. "featured", "emoji", "trend"
		},
	}

	params := allowedParams[endpoint]
	tenorURL := "https://tenor.googleapis.com/v2/" + endpoint + "?key=" + url.QueryEscape(apiKey) + "&client_key=" + url.QueryEscape(clientKey)

	for key, values := range r.URL.Query() {
		if key == "endpoint" {
			continue
		}
		if params != nil && !params[key] {
			continue // skip params not allowed for this endpoint
		}
		for _, value := range values {
			tenorURL += "&" + key + "=" + url.QueryEscape(value)
		}
	}

	// println("Tenor request URL:", tenorURL) // For debugging

	resp, err := http.Get(tenorURL)
	if err != nil {
		http.Error(w, "Failed to fetch from Tenor", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Log the error response for debugging
		println("Tenor error:", string(body))
		w.Write(body)
		return
	}

	_, copyErr := io.Copy(w, resp.Body)
	if copyErr != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
	}
}
