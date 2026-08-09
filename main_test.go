package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("HTTP_ADDR", "")
		if got := loadConfig().HTTPAddr; got != ":8080" {
			t.Fatalf("HTTPAddr = %q, want %q", got, ":8080")
		}
	})

	t.Run("environment", func(t *testing.T) {
		t.Setenv("HTTP_ADDR", " 127.0.0.1:9090 ")
		if got := loadConfig().HTTPAddr; got != "127.0.0.1:9090" {
			t.Fatalf("HTTPAddr = %q, want %q", got, "127.0.0.1:9090")
		}
	})
}

func TestInfoEndpoint(t *testing.T) {
	want := instanceInfo{
		InstanceID:   "hello.2.task",
		Hostname:     "container-2",
		IPAddresses:  []string{"10.0.1.12", "172.18.0.4"},
		Version:      "1.2.3",
		GoVersion:    "go1.24.0",
		OS:           "linux",
		Architecture: "arm64",
		StartedAt:    "2026-07-26T12:00:00Z",
		NodeName:     "worker-1",
		ServiceName:  "hello",
		TaskName:     "hello.2.task",
		TaskSlot:     "2",
	}

	request := httptest.NewRequest(http.MethodGet, "/api/info", nil)
	response := httptest.NewRecorder()
	newHandler(want).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if got := response.Header().Get("Connection"); got != "close" {
		t.Errorf("Connection = %q, want close", got)
	}
	if got := response.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("Content-Security-Policy is missing")
	}

	var got instanceInfo
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.InstanceID != want.InstanceID || got.Hostname != want.Hostname {
		t.Errorf("response identity = %q/%q, want %q/%q", got.InstanceID, got.Hostname, want.InstanceID, want.Hostname)
	}
	if strings.Join(got.IPAddresses, ",") != strings.Join(want.IPAddresses, ",") {
		t.Errorf("IP addresses = %v, want %v", got.IPAddresses, want.IPAddresses)
	}
}

func TestWebAndHealthEndpoints(t *testing.T) {
	handler := newHandler(instanceInfo{InstanceID: "test"})

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/", contentType: "text/html", contains: "Hello Swarm"},
		{path: "/app.js", contentType: "text/javascript", contains: "async function poll"},
		{path: "/healthz", contentType: "text/plain", contains: "ok\n"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Content-Type"); !strings.Contains(got, test.contentType) {
				t.Errorf("Content-Type = %q, want it to contain %q", got, test.contentType)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read response: %v", err)
			}
			if !strings.Contains(string(body), test.contains) {
				t.Errorf("response does not contain %q", test.contains)
			}
		})
	}
}

func TestInfoEndpointRejectsPost(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/info", nil)
	response := httptest.NewRecorder()
	newHandler(instanceInfo{}).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
