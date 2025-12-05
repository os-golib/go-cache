package metrics

// // BatchResult holds the result of a batch operation
// type BatchResult[T any] struct {
// 	Values map[string]T
// 	Errors map[string]error
// }

// // NewBatchResult returns a new batch result
// func NewBatchResult[T any]() *BatchResult[T] {
// 	return &BatchResult[T]{
// 		Values: make(map[string]T),
// 		Errors: make(map[string]error),
// 	}
// }

// // CacheableFunction is a function that can be cached
// type CacheableFunction[T any] func() (T, error)

// // WithFallback returns a function that returns the fallback value if the original function returns an error
// func WithFallback[T any](fn func() (T, error), fallback T) func() (T, error) {
// 	return func() (T, error) {
// 		val, err := fn()
// 		if err != nil {
// 			return fallback, nil
// 		}
// 		return val, err
// 	}
// }

// // WithRetry returns a function that retries the original function up to maxRetries times
// func WithRetry[T any](fn func() (T, error), maxRetries int) func() (T, error) {
// 	return func() (T, error) {
// 		var lastErr error
// 		for i := 0; i < maxRetries; i++ {
// 			val, err := fn()
// 			if err == nil {
// 				return val, nil
// 			}
// 			lastErr = err
// 		}
// 		var zero T
// 		return zero, lastErr
// 	}
// }
