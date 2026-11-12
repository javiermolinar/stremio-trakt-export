package stremio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLibraryResponseShapes(t *testing.T) {
	items := []LibraryItem{{ID: "tt123", Type: "movie"}}
	tests := map[string]func() any{
		"array": func() any { return map[string]any{"result": items} },
		"wrapped": func() any {
			return map[string]any{"result": map[string]any{"library": items}}
		},
	}
	for name, response := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/datastoreGet" {
					t.Errorf("path = %q", r.URL.Path)
				}
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if request["authKey"] != "secret" || request["collection"] != "libraryItem" || request["all"] != true {
					t.Errorf("unexpected request: %#v", request)
				}
				_ = json.NewEncoder(w).Encode(response())
			}))
			defer server.Close()
			client := Client{APIURL: server.URL + "/api", HTTPClient: server.Client()}
			got, err := client.Library(context.Background(), "secret")
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].ID != "tt123" {
				t.Fatalf("unexpected library: %#v", got)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/login" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var request map[string]string
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["email"] != "user@example.com" || request["password"] != "password" {
			t.Errorf("unexpected login request")
		}
		_, _ = w.Write([]byte(`{"result":{"authKey":"returned-key"}}`))
	}))
	defer server.Close()
	client := Client{APIURL: server.URL + "/api", HTTPClient: server.Client()}
	authKey, err := client.Login(context.Background(), "user@example.com", "password")
	if err != nil {
		t.Fatal(err)
	}
	if authKey != "returned-key" {
		t.Fatalf("authKey = %q", authKey)
	}
}

func TestLibraryAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid authKey"}}`))
	}))
	defer server.Close()
	client := Client{APIURL: server.URL, HTTPClient: server.Client()}
	if _, err := client.Library(context.Background(), "secret"); err == nil {
		t.Fatal("expected authentication error")
	}
}
