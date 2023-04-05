package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// ErrBlocked reports if service is blocked.
var ErrBlocked = errors.New("blocked")

// Service defines external service that can process batches of items.
type Service interface {
	GetLimits() (n uint64, p time.Duration)
	Process(ctx context.Context, batch Batch) error
}

// Batch is a batch of items.
type Batch []Item

// Item is some abstract item.
type Item struct{}

// Client is a client to the external service.
type Client struct {
	service Service
}

// NewClient creates a new client to the external service.
func NewClient(service Service) *Client {
	return &Client{service: service}
}

// ProcessItems processes items by the external service.
func (c *Client) ProcessItems(ctx context.Context, items []Item) error {
	n, p := c.service.GetLimits()
	batchSize := int(n)

	if len(items) < batchSize {
		batchSize = len(items)
	}

	startTime := time.Now()
	for i := 0; i < len(items); i += batchSize {
		if time.Since(startTime) >= p {
			return ErrBlocked
		}

		batch := items[i:min(i+batchSize, len(items))]
		err := c.service.Process(ctx, batch)
		if err != nil {
			return err
		}
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// API is an API for interacting with the client.
type API struct {
	client *Client
}

// NewAPI creates a new API for interacting with the client.
func NewAPI(client *Client) *API {
	return &API{client: client}
}

// ProcessItemsHandler is a handler for processing items.
func (a *API) ProcessItemsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request body to get items.
	items := make([]Item, 0)
	err := json.NewDecoder(r.Body).Decode(&items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Process items.
	ctx := r.Context()
	err = a.client.ProcessItems(ctx, items)
	if err != nil {
		if err == ErrBlocked {
			http.Error(w, err.Error(), http.StatusTooManyRequests)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Send response.
	w.WriteHeader(http.StatusOK)
}

func main() {
}
