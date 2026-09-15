package provider

import (
	"context"
	"sync"
)

// Fake is an in-memory Provider for tests.
type Fake struct {
	mu     sync.Mutex
	NameV  string
	Calls  []ChatRequest
	ChatFn func(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// Name implements Provider.
func (f *Fake) Name() string {
	if f.NameV == "" {
		return "fake"
	}
	return f.NameV
}

// Chat implements Provider.
func (f *Fake) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, req)
	fn := f.ChatFn
	f.mu.Unlock()
	if fn != nil {
		return fn(ctx, req)
	}
	return ChatResponse{}, nil
}
