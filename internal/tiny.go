package internal

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

var apiKey string

func InitTinify(tinifyApiKey string) {
	apiKey = tinifyApiKey
}

func Tinify(source string) (result []byte, err error) {
	if apiKey == "" {
		return nil, errors.New("Tinify not initialised. Call `InitTinify` first.")
	}
	compressed := compress(source)
	result = convert(compressed)
	return
}

func compress(source string) (output string) {
	client := &http.Client{}
	jsonBody := []byte(fmt.Sprintf(`{"source":{"url":"%s"}}`, source))
	bodyReader := bytes.NewReader(jsonBody)
	req, _ := http.NewRequest("POST", "https://api.tinify.com/shrink", bodyReader)
	req.SetBasicAuth("api", apiKey)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	return resp.Header["Location"][0]
}

func convert(source string) (output []byte) {
	client := &http.Client{}
	jsonBody := []byte(`{"resize":{"method":"scale","width":1000},"convert":{"type":"image/webp"}}`)
	bodyReader := bytes.NewReader(jsonBody)
	req, _ := http.NewRequest("POST", source, bodyReader)
	req.SetBasicAuth("api", apiKey)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return body
}
