// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestClient_EdgeCasesAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("FabricAPIError and NotFoundError string representations", func(t *testing.T) {
		t.Parallel()

		nfDefault := &NotFoundError{
			ResourceType: "Warehouse",
			ResourceID:   "wh-123",
			WorkspaceID:  "ws-456",
		}
		if nfDefault.Error() != `Warehouse "wh-123" not found in workspace "ws-456"` {
			t.Errorf("unexpected default NotFoundError format: %s", nfDefault.Error())
		}

		nfCustom := &NotFoundError{
			Message: "custom not found message",
		}
		if nfCustom.Error() != "custom not found message" {
			t.Errorf("unexpected custom NotFoundError: %s", nfCustom.Error())
		}

		if !IsNotFound(nfDefault) {
			t.Error("expected IsNotFound(NotFoundError) to be true")
		}

		apiErr404 := &APIError{
			StatusCode: http.StatusNotFound,
			ErrorCode:  "ItemNotFound",
			Message:    "item does not exist",
		}
		if !IsNotFound(apiErr404) {
			t.Error("expected IsNotFound(APIError 404) to be true")
		}
		if apiErr404.Error() != "fabric API error (HTTP 404, code ItemNotFound): item does not exist" {
			t.Errorf("unexpected APIError formatting: %s", apiErr404.Error())
		}

		apiErr500 := &APIError{
			StatusCode: http.StatusInternalServerError,
			ErrorCode:  "InternalServerError",
			Message:    "server failure",
		}
		if IsNotFound(apiErr500) {
			t.Error("expected IsNotFound(APIError 500) to be false")
		}
		if IsNotFound(errors.New("generic error")) {
			t.Error("expected IsNotFound(generic error) to be false")
		}
	})

	t.Run("ToAPIPermission conversions", func(t *testing.T) {
		t.Parallel()

		perms := map[string]string{
			"read":                     "Read",
			"read_data":                "ReadData",
			"readdata":                 "ReadData",
			"read_spark":               "ReadAll",
			"readall":                  "ReadAll",
			"read_all":                 "ReadAll",
			"subscribe_onelake_events": "SubscribeOneLakeEvents",
			"subscribeonelakeevents":   "SubscribeOneLakeEvents",
			"write":                    "Write",
			"reshare":                  "Reshare",
		}
		for in, expected := range perms {
			got, err := ToAPIPermission(in)
			if err != nil || got != expected {
				t.Errorf("ToAPIPermission(%q) = %q, %v; expected %q", in, got, err, expected)
			}
		}

		if _, err := ToAPIPermission("unknown_perm"); err == nil {
			t.Error("expected error for unknown permission")
		}
	})

	t.Run("ExpandRolePermissions edge cases", func(t *testing.T) {
		t.Parallel()

		if _, err := ExpandRolePermissions("Warehouse", "read_data"); err == nil {
			t.Error("expected Warehouse read_data to be rejected")
		}
		if _, err := ExpandRolePermissions("Warehouse", "read_spark"); err == nil {
			t.Error("expected Warehouse read_spark to be rejected")
		}
		if _, err := ExpandRolePermissions("Warehouse", "unknown_role"); err == nil {
			t.Error("expected unknown role_type to be rejected")
		}

		lakehouseSpark, err := ExpandRolePermissions("Lakehouse", "read_spark")
		if err != nil || len(lakehouseSpark) != 3 {
			t.Errorf("unexpected lakehouse read_spark expansion: %v, %v", lakehouseSpark, err)
		}
		lakehouseData, err := ExpandRolePermissions("Lakehouse", "read_data")
		if err != nil || len(lakehouseData) != 2 {
			t.Errorf("unexpected lakehouse read_data expansion: %v, %v", lakehouseData, err)
		}
	})

	t.Run("BuildLakehouseRole defaults", func(t *testing.T) {
		t.Parallel()

		role := BuildLakehouseRole("ws", "lh", "test-role", nil, nil, "tenant-id", "principal-id", "User")
		if len(role.DecisionRules) == 0 {
			t.Fatal("expected decision rules")
		}
		scopes := role.DecisionRules[0].Permission
		if len(scopes) != 2 {
			t.Fatalf("expected 2 scopes, got %d", len(scopes))
		}
		if scopes[0].AttributeValueIncludedIn[0] != "*" {
			t.Errorf("expected default path '*', got %v", scopes[0].AttributeValueIncludedIn)
		}
		if scopes[1].AttributeValueIncludedIn[0] != "Read" {
			t.Errorf("expected default action 'Read', got %v", scopes[1].AttributeValueIncludedIn)
		}
	})

	t.Run("retryDelay and shouldRetryStatus", func(t *testing.T) {
		t.Parallel()

		c := &FabricClient{
			baseBackoff: 100 * time.Millisecond,
		}

		delayHeader := c.retryDelay("3", 1)
		if delayHeader != 3*time.Second {
			t.Errorf("expected 3s delay from header, got %v", delayHeader)
		}

		cFast := &FabricClient{
			baseBackoff: 10 * time.Millisecond,
		}
		delayCapped := cFast.retryDelay("3", 1)
		if delayCapped != 30*time.Millisecond {
			t.Errorf("expected 30ms capped delay, got %v", delayCapped)
		}

		delayBackoff := c.retryDelay("", 0)
		if delayBackoff < 100*time.Millisecond {
			t.Errorf("expected backoff >= 100ms, got %v", delayBackoff)
		}

		if !shouldRetryStatus(http.StatusTooManyRequests) {
			t.Error("expected 429 to retry")
		}
		if !shouldRetryStatus(http.StatusBadGateway) {
			t.Error("expected 502 to retry")
		}
		if !shouldRetryStatus(http.StatusServiceUnavailable) {
			t.Error("expected 503 to retry")
		}
		if !shouldRetryStatus(http.StatusGatewayTimeout) {
			t.Error("expected 504 to retry")
		}
		if shouldRetryStatus(http.StatusBadRequest) {
			t.Error("expected 400 not to retry")
		}
	})

	t.Run("sleepWithContext cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := sleepWithContext(ctx, 1*time.Minute)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("doJSON response body read error and invalid JSON decode", func(t *testing.T) {
		t.Parallel()

		// 1. Broken reader for io.ReadAll error
		brokenBodyClient, err := NewFabricClient(Config{
			Endpoint: "https://api.fabric.microsoft.com",
			TokenProvider: func(context.Context) (string, error) {
				return "mock-token", nil
			},
			HTTPClient: &http.Client{
				Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(&errReader{}),
						Header:     make(http.Header),
					}, nil
				}),
			},
		})
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		var out any
		_, readErr := brokenBodyClient.doJSON(context.Background(), http.MethodGet, "/test", nil, nil, &out)
		if readErr == nil {
			t.Fatal("expected error from broken body reader")
		}

		// 2. Invalid JSON decode for 200 OK
		invalidJSONClient, err := NewFabricClient(Config{
			Endpoint: "https://api.fabric.microsoft.com",
			TokenProvider: func(context.Context) (string, error) {
				return "mock-token", nil
			},
			HTTPClient: &http.Client{
				Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte("{invalid-json"))),
						Header:     make(http.Header),
					}, nil
				}),
			},
		})
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		_, decodeErr := invalidJSONClient.doJSON(context.Background(), http.MethodGet, "/test", nil, nil, &out)
		if decodeErr == nil {
			t.Fatal("expected JSON decode error")
		}

		// 3. Invalid payload marshal in doJSON (channel type)
		_, marshalErr := invalidJSONClient.doJSON(context.Background(), http.MethodPost, "/test", nil, make(chan int), nil)
		if marshalErr == nil {
			t.Fatal("expected error marshalling channel in doJSON")
		}
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type errReader struct{}

func (e *errReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated socket read failure")
}
