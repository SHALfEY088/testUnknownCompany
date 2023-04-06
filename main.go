package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	n       uint64
	p       time.Duration
	queue   chan Batch
}

// NewClient creates a new client to the external service.
func NewClient(service Service) *Client {
	n, p := service.GetLimits()
	return &Client{
		service: service,
		n:       n,
		p:       p,
		queue:   make(chan Batch),
	}
}

// ProcessItems processes items by the external service.
func (c *Client) Process(batch Batch) {
	c.queue <- batch
}

// an infinite loop of data processing from the queue queue with the given restrictions.
func (c *Client) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-c.queue:
			go func() {
				ticker := time.NewTicker(c.p)
				defer ticker.Stop()

				for i := uint64(0); i < uint64(len(batch)); i += c.n {
					end := i + c.n
					if end > uint64(len(batch)) {
						end = uint64(len(batch))
					}

					subBatch := batch[i:end]
					err := c.service.Process(ctx, subBatch)
					if err != nil {
						log.Printf("Error processing subBatch (retry %d): %v", i+1, err)
					}

					<-ticker.C
				}
			}()
		}
	}
}

func handleRequest(client *Client, w http.ResponseWriter, r *http.Request) {
	batch, err := convertRequestToBatch(r)
	if err != nil {
		http.Error(w, "convert request to batch error", http.StatusBadRequest)
	}
	client.Process(batch)
	w.WriteHeader(http.StatusOK)
}

func main() {
	// Create an external service (e.g. dummyService)
	// This assumes that dummyService implements the Service interface
	externalService := &dummyService{
		n: 10, // the number of items the service can handle
		p: time.Second * 2, // element processing time interval
	}

	client := NewClient(externalService)

	// Run the client's Run method in a separate goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.Run(ctx)

	http.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(client, w, r)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))

	// curl -X POST -H "Content-Type: application/json" -d '[1, 2, 3, 4, 5]' http://localhost:8080/process
	// Processed batch of 5 items
}

type dummyService struct {
	n uint64
	p time.Duration
}

func (s *dummyService) GetLimits() (uint64, time.Duration) {
	return s.n, s.p
}

func (s *dummyService) Process(ctx context.Context, batch Batch) error {
	time.Sleep(s.p)
	fmt.Printf("Processed batch of %d items\n", len(batch))
	return nil
}

func NewDummyService(n uint64, p time.Duration) *dummyService {
	return &dummyService{
		n: n,
		p: p,
	}
}


func convertRequestToBatch(r *http.Request) (Batch, error) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)

	var items []int
	err := decoder.Decode(&items)
	if err != nil {
		return nil, err
	}

	batch := make(Batch, len(items))
	for i := range items {
		batch[i] = Item{}
	}

	return batch, nil
}
