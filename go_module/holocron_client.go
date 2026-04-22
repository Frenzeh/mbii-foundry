package main

// Dev-only integration with the internal MBII Holocron server.
//
// Regular users will never touch this. It exists so MBII maintainers can:
//
//  1. Query the Holocron's RAG brain from the info panel while editing —
//     adds a contextual "insight" blurb pulled from source code knowledge.
//  2. Push local file changes into a running Holocron for definition
//     refresh workflows (the server-side `fa_generate_definitions` tool
//     reads source headers and emits updated JSON/markdown stubs into
//     this repo; the client here is the UI-side companion that helps
//     maintainers shepherd those stubs through review).
//
// The integration is gated on the `MBII_FOUNDRY_DEV=1` environment
// variable. Without it:
//   - The client is never constructed.
//   - No network calls are made.
//   - The "Share" button and Holocron status icon are hidden.
//   - Info-panel prose comes purely from the bundled `definitions/*.md`
//     files.
//
// Keeping the code in the public repo (instead of a separate maintainer-
// only branch) is deliberate: it avoids drift and makes the integration
// visible to anyone auditing the app's network behavior. But users should
// not encounter any Holocron-branded UI. If you're adding a new Holocron
// touchpoint, follow the same pattern — check `os.Getenv("MBII_FOUNDRY_DEV")`
// before surfacing anything Holocron-related in the UI.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// HolocronClient talks to a local Holocron Ops server for maintainer
// workflows. See file header for context; this struct is dev-only.
type HolocronClient struct {
	BaseURL   string
	Available bool
	Client    *http.Client
}

// NewHolocronClient returns a client only when MBII_FOUNDRY_DEV is set in
// the environment. Returns nil for regular users — every caller MUST
// nil-check.
func NewHolocronClient() *HolocronClient {
	if os.Getenv("MBII_FOUNDRY_DEV") == "" {
		return nil
	}
	return &HolocronClient{
		BaseURL: "http://localhost:18080",
		Client: &http.Client{
			Timeout: 500 * time.Millisecond,
		},
	}
}

// CheckAvailability pings the server to see if it's online. Safe to call
// on a nil receiver (returns false).
func (hc *HolocronClient) CheckAvailability() bool {
	if hc == nil {
		return false
	}
	resp, err := hc.Client.Get(hc.BaseURL + "/api/stats")
	if err != nil {
		hc.Available = false
		return false
	}
	defer resp.Body.Close()

	hc.Available = resp.StatusCode == 200
	return hc.Available
}

// AskResult represents the JSON response from the Holocron API.
type AskResult struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// Ask sends a query to the Holocron Brain.
func (hc *HolocronClient) Ask(query string) (string, error) {
	if hc == nil || !hc.Available {
		return "", fmt.Errorf("holocron offline")
	}

	url := fmt.Sprintf("%s/api/chat", hc.BaseURL)
	payload := map[string]string{"message": query}
	jsonPayload, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("server error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result AskResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Error != "" {
		return "", fmt.Errorf("api error: %s", result.Error)
	}

	return result.Response, nil
}

// ShareFile uploads a file to the Holocron server (maintainer workflow:
// shepherd a candidate definition change through Holocron's review tools
// before it lands in the repo).
func (hc *HolocronClient) ShareFile(filename string, content string, fileType string) (string, error) {
	if hc == nil || !hc.Available {
		return "", fmt.Errorf("holocron offline")
	}

	url := fmt.Sprintf("%s/api/share", hc.BaseURL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	part.Write([]byte(content))

	writer.WriteField("type", fileType)
	writer.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("upload failed: %s", string(respBody))
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if msg, ok := result["message"].(string); ok {
		return msg, nil
	}
	return "Upload successful", nil
}
