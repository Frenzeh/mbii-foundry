package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// HolocronClient handles communication with the local Python Holocron Ops server.
type HolocronClient struct {
	BaseURL   string
	Available bool
	Client    *http.Client
}

// NewHolocronClient creates a new client pointing to the local server.
func NewHolocronClient() *HolocronClient {
	return &HolocronClient{
		BaseURL: "http://localhost:18080", // Correct port for Holocron System
		Client: &http.Client{
			Timeout: 500 * time.Millisecond, // Slightly relaxed timeout
		},
	}
}

// CheckAvailability pings the server to see if it's online.
func (hc *HolocronClient) CheckAvailability() bool {
	resp, err := hc.Client.Get(hc.BaseURL + "/api/stats") // Use a lightweight API endpoint
	if err != nil {
		hc.Available = false
		return false
	}
	defer resp.Body.Close()

	hc.Available = resp.StatusCode == 200
	return hc.Available
}

// AskResult represents the JSON response from the Python API.
type AskResult struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// Ask sends a query to the Holocron Brain.
func (hc *HolocronClient) Ask(query string) (string, error) {
	if !hc.Available {
		return "", fmt.Errorf("holocron offline")
	}

	url := fmt.Sprintf("%s/api/chat", hc.BaseURL)
	payload := map[string]string{"message": query}
	jsonPayload, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 5 * time.Second} // Longer timeout for AI
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

// ShareFile uploads a file to the Holocron server.
func (hc *HolocronClient) ShareFile(filename string, content string, fileType string) (string, error) {
	if !hc.Available {
		return "", fmt.Errorf("holocron offline")
	}

	url := fmt.Sprintf("%s/api/share", hc.BaseURL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file content
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	part.Write([]byte(content))

	// Add file type
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
