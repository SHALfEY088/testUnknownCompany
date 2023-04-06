package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testService struct {
	n uint64
	p time.Duration
}

func (s *testService) GetLimits() (uint64, time.Duration) {
	return s.n, s.p
}

func (s *testService) Process(ctx context.Context, batch Batch) error {
	if len(batch) == 0 {
		return errors.New("empty batch")
	}

	time.Sleep(s.p)
	fmt.Printf("Processed batch of %d items\n", len(batch))
	return nil
}

func TestClientRun(t *testing.T) {
	service := &testService{
		n: 2,
		p: time.Millisecond * 50,
	}

	client := NewClient(service)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()

	go client.Run(ctx)

	batch := make(Batch, 4)
	for i := range batch {
		batch[i] = Item{}
	}
	client.Process(batch)

	<-ctx.Done()
}

func TestConvertRequestToBatch(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	data, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/process", bytes.NewBuffer(data))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")

	batch, err := convertRequestToBatch(req)
	if err != nil {
		t.Fatal(err)
	}

	if len(batch) != len(items) {
		t.Fatalf("expected %d items, got %d", len(items), len(batch))
	}
}

func TestHandleRequest(t *testing.T) {
	service := NewDummyService(2, time.Millisecond*50)
	client := NewClient(service)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()

	go client.Run(ctx)

	items := []int{1, 2, 3, 4, 5}
	data, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/process", bytes.NewBuffer(data))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleRequest(client, w, r)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}
