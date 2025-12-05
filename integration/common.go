package integration

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"time"
)

// Common defaults
const (
	DefaultTimeout       = 3 * time.Second
	DefaultRetryAttempts = 3
	DefaultRetryDelay    = 100 * time.Millisecond
	DefaultBatchSize     = 100
	DefaultWorkerCount   = 10
)

// Options provides common configuration for integrations
type Options struct {
	Timeout        time.Duration
	RetryAttempts  int
	RetryDelay     time.Duration
	AsyncExecution bool
}

// DefaultOptions returns default integration options
func DefaultOptions() Options {
	return Options{
		Timeout:        DefaultTimeout,
		RetryAttempts:  DefaultRetryAttempts,
		RetryDelay:     DefaultRetryDelay,
		AsyncExecution: true,
	}
}

// safeExec executes a function safely with panic recovery
func safeExec(name string, fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[go-cache][%s] panic: %v\n%s", name, r, debug.Stack())
		}
	}()

	if err := fn(); err != nil {
		log.Printf("[go-cache][%s] error: %v", name, err)
	}
}

// safeExecWithResult executes a function with result and panic recovery
func safeExecWithResult[T any](name string, fn func() (T, error)) (result T, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[go-cache][%s] panic: %v\n%s", name, r, debug.Stack())
			err = &PanicError{Name: name, Reason: r}
		}
	}()

	return fn()
}

// executeAsync executes a function asynchronously with error logging
func executeAsync(name string, fn func() error) {
	go safeExec(name, fn)
}

// executeWithTimeout executes a function with a timeout
func executeWithTimeout[T any](ctx context.Context, timeout time.Duration, fn func() (T, error)) (T, error) {
	var zero T

	if timeout <= 0 {
		return fn()
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resultChan := make(chan struct {
		Result T
		Error  error
	}, 1)

	go func() {
		result, err := safeExecWithResult("timeout_exec", fn)
		resultChan <- struct {
			Result T
			Error  error
		}{Result: result, Error: err}
	}()

	select {
	case result := <-resultChan:
		return result.Result, result.Error
	case <-ctx.Done():
		return zero, &TimeoutError{Operation: "function", Timeout: timeout}
	}
}

// PanicError represents a panic during execution
type PanicError struct {
	Name   string
	Reason any
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("panic in %s: %v", e.Name, e.Reason)
}

// TimeoutError represents a timeout error
type TimeoutError struct {
	Operation string
	Timeout   time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s timed out after %v", e.Operation, e.Timeout)
}

func (e *TimeoutError) Temporary() bool { return true }

// BatchProcessor processes items in batches with concurrency control
type BatchProcessor[T any] struct {
	batchSize int
	workers   int
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor[T any](batchSize, workers int) *BatchProcessor[T] {
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	if workers <= 0 {
		workers = DefaultWorkerCount
	}

	return &BatchProcessor[T]{
		batchSize: batchSize,
		workers:   workers,
	}
}

// Process processes items in batches
func (bp *BatchProcessor[T]) Process(items []T, processor func([]T) error) error {
	if len(items) == 0 {
		return nil
	}

	batches := bp.createBatches(items)
	errChan := make(chan error, len(batches))
	sem := make(chan struct{}, bp.workers)

	for _, batch := range batches {
		sem <- struct{}{}
		go func(b []T) {
			defer func() { <-sem }()
			if err := processor(b); err != nil {
				errChan <- err
			}
		}(batch)
	}

	// Wait for completion
	for i := 0; i < bp.workers; i++ {
		sem <- struct{}{}
	}
	close(errChan)

	// Return first error
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (bp *BatchProcessor[T]) createBatches(items []T) [][]T {
	numBatches := (len(items) + bp.batchSize - 1) / bp.batchSize
	batches := make([][]T, 0, numBatches)

	for i := 0; i < len(items); i += bp.batchSize {
		end := min(i+bp.batchSize, len(items))
		batches = append(batches, items[i:end])
	}

	return batches
}

// ParallelProcessor processes items in parallel
type ParallelProcessor[T any, R any] struct {
	workers int
}

// NewParallelProcessor creates a new parallel processor
func NewParallelProcessor[T any, R any](workers int) *ParallelProcessor[T, R] {
	if workers <= 0 {
		workers = DefaultWorkerCount
	}
	return &ParallelProcessor[T, R]{workers: workers}
}

// Process processes items in parallel and returns results
func (pp *ParallelProcessor[T, R]) Process(items []T, processor func(T) (R, error)) ([]R, error) {
	if len(items) == 0 {
		return []R{}, nil
	}

	type result struct {
		value R
		index int
		err   error
	}

	results := make([]R, len(items))
	resultChan := make(chan result, len(items))
	sem := make(chan struct{}, pp.workers)

	for i, item := range items {
		sem <- struct{}{}
		go func(idx int, itm T) {
			defer func() { <-sem }()
			value, err := processor(itm)
			resultChan <- result{value: value, index: idx, err: err}
		}(i, item)
	}

	// Wait for completion
	for i := 0; i < pp.workers; i++ {
		sem <- struct{}{}
	}
	close(resultChan)

	var firstError error
	for res := range resultChan {
		if res.err != nil && firstError == nil {
			firstError = res.err
		}
		results[res.index] = res.value
	}

	return results, firstError
}
