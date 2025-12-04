package integration

import (
	"log"
	"runtime/debug"
	"time"
)

// DefaultTimeout is the default timeout for integration operations
var DefaultTimeout = 3 * time.Second

// DefaultRetryAttempts is the default number of retry attempts
var DefaultRetryAttempts = 3

// DefaultRetryDelay is the default delay between retries
var DefaultRetryDelay = 100 * time.Millisecond

// safeExec executes a function safely, logging any panics or errors
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

// safeExecWithResult executes a function that returns a value, handling panics
func safeExecWithResult[T any](name string, fn func() (T, error)) (result T, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[go-cache][%s] panic: %v\n%s", name, r, debug.Stack())
			err = &PanicError{Name: name, Reason: r}
		}
	}()

	return fn()
}

// // retry executes a function with retry logic
// func retry(attempts int, delay time.Duration, fn func() error) error {
// 	var err error
// 	for i := 0; i < attempts; i++ {
// 		err = fn()
// 		if err == nil {
// 			return nil
// 		}

// 		if i < attempts-1 {
// 			time.Sleep(delay)
// 			delay *= 2 // Exponential backoff
// 		}
// 	}
// 	return err
// }

// // retryWithResult executes a function with retry logic that returns a value
// func retryWithResult[T any](attempts int, delay time.Duration, fn func() (T, error)) (T, error) {
// 	var result T
// 	var err error

// 	for i := 0; i < attempts; i++ {
// 		result, err = fn()
// 		if err == nil {
// 			return result, nil
// 		}

// 		if i < attempts-1 {
// 			time.Sleep(delay)
// 			delay *= 2 // Exponential backoff
// 		}
// 	}
// 	return result, err
// }

// PanicError represents a panic that occurred during execution
type PanicError struct {
	Name   string
	Reason any
}

func (e *PanicError) Error() string {
	return "panic in " + e.Name + ": " + e.String()
}

func (e *PanicError) String() string {
	switch r := e.Reason.(type) {
	case string:
		return r
	case error:
		return r.Error()
	default:
		return "unknown panic"
	}
}

// IsRecoverableError checks if an error is recoverable (should be retried)
func IsRecoverableError(err error) bool {
	if err == nil {
		return false
	}

	// Add logic to determine if error is recoverable
	// For example, network errors, timeouts, etc.
	switch err.(type) {
	case *PanicError:
		return false
	default:
		return true
	}
}

// ExecuteWithTimeout executes a function with a timeout
func ExecuteWithTimeout(timeout time.Duration, fn func() error) error {
	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- &PanicError{Name: "timeout_exec", Reason: r}
			}
		}()
		done <- fn()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return &TimeoutError{Operation: "function", Timeout: timeout}
	}
}

// ExecuteWithTimeoutAndResult executes a function with timeout that returns a value
func ExecuteWithTimeoutAndResult[T any](timeout time.Duration, fn func() (T, error)) (T, error) {
	var zero T
	resultChan := make(chan struct {
		Result T
		Error  error
	}, 1)

	go func() {
		defer func() {
			if rcr := recover(); rcr != nil {
				resultChan <- struct {
					Result T
					Error  error
				}{
					Result: zero,
					Error:  &PanicError{Name: "timeout_exec_result", Reason: rcr},
				}
			}
		}()
		result, err := fn()
		resultChan <- struct {
			Result T
			Error  error
		}{Result: result, Error: err}
	}()

	select {
	case result := <-resultChan:
		return result.Result, result.Error
	case <-time.After(timeout):
		return zero, &TimeoutError{Operation: "function", Timeout: timeout}
	}
}

// TimeoutError represents a timeout error
type TimeoutError struct {
	Operation string
	Timeout   time.Duration
}

func (e *TimeoutError) Error() string {
	return e.Operation + " timed out after " + e.Timeout.String()
}

// TimeoutError returns the timeout duration
func (e *TimeoutError) TimeoutError() time.Duration {
	return e.Timeout
}

// Temporary indicates if the error is temporary
func (e *TimeoutError) Temporary() bool {
	return true
}

// BatchProcessor processes items in batches
type BatchProcessor[T any] struct {
	BatchSize int
	Workers   int
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor[T any](batchSize, workers int) *BatchProcessor[T] {
	return &BatchProcessor[T]{
		BatchSize: batchSize,
		Workers:   workers,
	}
}

// Process processes items in batches using multiple workers
func (bp *BatchProcessor[T]) Process(
	items []T,
	processor func([]T) error,
) error {
	if len(items) == 0 {
		return nil
	}

	batches := bp.createBatches(items)
	errorChan := make(chan error, len(batches))
	sem := make(chan struct{}, bp.Workers)

	for _, batch := range batches {
		sem <- struct{}{}
		go func(batch []T) {
			defer func() { <-sem }()
			if err := processor(batch); err != nil {
				errorChan <- err
			}
		}(batch)
	}

	// Wait for all workers to complete
	for i := 0; i < bp.Workers; i++ {
		sem <- struct{}{}
	}

	close(errorChan)

	// Return first error if any
	for err := range errorChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// createBatches splits items into batches
func (bp *BatchProcessor[T]) createBatches(items []T) [][]T {
	batches := make([][]T, 0, (len(items)+bp.BatchSize-1)/bp.BatchSize)

	for i := 0; i < len(items); i += bp.BatchSize {
		end := i + bp.BatchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}

// ParallelProcessor processes items in parallel
type ParallelProcessor[T any, R any] struct {
	Workers int
}

// NewParallelProcessor creates a new parallel processor
func NewParallelProcessor[T any, R any](workers int) *ParallelProcessor[T, R] {
	return &ParallelProcessor[T, R]{Workers: workers}
}

// Process processes items in parallel and returns results
func (pp *ParallelProcessor[T, R]) Process(
	items []T,
	processor func(T) (R, error),
) ([]R, error) {
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
	sem := make(chan struct{}, pp.Workers)

	for in, item := range items {
		sem <- struct{}{}
		go func(i int, itm T) {
			defer func() { <-sem }()
			value, err := safeExecWithResult("parallel_process", func() (R, error) {
				return processor(itm)
			})
			resultChan <- result{value: value, index: i, err: err}
		}(in, item)
	}

	// Wait for all workers to complete
	for i := 0; i < pp.Workers; i++ {
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
