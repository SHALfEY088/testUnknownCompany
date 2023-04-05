package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type mockService struct {
	n uint64
	p time.Duration
}

func (s *mockService) GetLimits() (n uint64, p time.Duration) {
	return s.n, s.p
}

func (s *mockService) Process(ctx context.Context, batch Batch) error {
	// Simulate some processing time.
	time.Sleep(100 * time.Millisecond)
	return nil
}

func TestClient_ProcessItems(t *testing.T) {
	tests := []struct {
		name        string
		service     Service
		items       []Item
		expectedErr error
	}{
		{
			name: "process all items",
			service: &mockService{
				n: 2,
				p: time.Second,
			},
			items:       []Item{{}, {}, {}, {}},
			expectedErr: nil,
		},
		{
			name: "block processing",
			service: &mockService{
				n: 2,
				p: time.Millisecond,
			},
			items:       []Item{{}, {}, {}, {}},
			expectedErr: ErrBlocked,
		},
		{
			name: "empty batch",
			service: &mockService{
				n: 2,
				p: time.Second,
			},
			items:       []Item{},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.service)

			ctx := context.Background()
			err := client.ProcessItems(ctx, tt.items)

			if err != tt.expectedErr {
				t.Errorf("expected error %v, but got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestAPI_ProcessItemsHandler(t *testing.T) {
	tests := []struct {
		name           string
		service        Service
		requestBody    string
		expectedStatus int
		expectedBody   string
		expectedErr    error
	}{
		{
			name: "process all items",
			service: &mockService{
				n: 2,
				p: time.Second,
			},
			requestBody:    "[{}, {}, {}, {}]",
			expectedStatus: http.StatusOK,
			expectedBody:   "",
			expectedErr:    nil,
		},
		{
			name: "block processing",
			service: &mockService{
				n: 2,
				p: time.Millisecond,
			},
			requestBody:    "[{}, {}, {}, {}]",
			expectedStatus: http.StatusTooManyRequests,
			expectedBody:   "blocked\n",
			expectedErr:    ErrBlocked,
		},
		{
			name: "bad request",
			service: &mockService{
				n: 2,
				p: time.Second,
			},
			requestBody:    "invalid",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid character 'i' looking for beginning of value\n",
			expectedErr:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.service)
			api := NewAPI(client)

			handler := http.HandlerFunc(api.ProcessItemsHandler)

			req, err := http.NewRequest("POST", "/items", strings.NewReader(tt.requestBody))
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %v, but got %v", tt.expectedStatus, rr.Code)
			}

			if rr.Body.String() != tt.expectedBody {
				t.Errorf("expected body %v, but got %v", tt.expectedBody, rr.Body.String())
			}
		})
	}
}
