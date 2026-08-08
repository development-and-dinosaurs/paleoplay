package internal

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var apiKey string

func InitTinify(tinifyApiKey string) {
	apiKey = tinifyApiKey
}

func Tinify(source string) (result []byte, err error) {
	compressed, err := compress(source)
	if err != nil {
		return nil, err
	}
	return convert(compressed)
}

func compress(source string) (output string, err error) {
	jsonBody := []byte(fmt.Sprintf(`{"source":{"url":"%s"}}`, source))
	bodyReader := bytes.NewReader(jsonBody)
	req, _ := http.NewRequest("POST", "https://api.tinify.com/shrink", bodyReader)
	req.SetBasicAuth("api", apiKey)
	req.Header.Add("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not compress %s: %w", source, err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, "compress "+source); err != nil {
		return "", err
	}
	// Tinify returns the compressed image as a Location header rather than a body.
	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("could not compress %s: no Location header", source)
	}
	return location, nil
}

func convert(source string) (output []byte, err error) {
	jsonBody := []byte(`{"convert":{"type":"image/webp"}}`)
	bodyReader := bytes.NewReader(jsonBody)
	req, _ := http.NewRequest("POST", source, bodyReader)
	req.SetBasicAuth("api", apiKey)
	req.Header.Add("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not convert %s: %w", source, err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, "convert "+source); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read converted %s: %w", source, err)
	}
	return body, nil
}

func checkStatus(resp *http.Response, action string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("could not %s: status %d: %s", action, resp.StatusCode, strings.TrimSpace(string(body)))
}
