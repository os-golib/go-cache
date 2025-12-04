package config

import "errors"

// Common cache errors
var (
	ErrKeyEmpty         = errors.New("empty key")
	ErrCacheMiss        = errors.New("cache miss")
	ErrInvalidConfig    = errors.New("invalid config")
	ErrSerialization    = errors.New("serialization error")
	ErrDeserialization  = errors.New("deserialization error")
	ErrTimeout          = errors.New("timeout")
	ErrConnectionFailed = errors.New("connection failed")
	ErrLockAcquisition  = errors.New("lock acquisition failed")
	ErrLockNotHeld      = errors.New("lock not held")
)

// Error represents a cache error with context
type Error struct {
	Op  string
	Err error
	Key string
}

// Error returns a string representation of the error
func (e *Error) Error() string {
	if e.Key != "" {
		return e.Op + " for key '" + e.Key + "': " + e.Err.Error()
	}
	return e.Op + ": " + e.Err.Error()
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Err
}

// NewError creates a new cache error with context
func NewError(op string, err error, key string) error {
	return &Error{
		Op:  op,
		Err: err,
		Key: key,
	}
}

// IsCacheMiss checks if an error is a cache miss
func IsCacheMiss(err error) bool {
	return errors.Is(err, ErrCacheMiss)
}

// IsTimeout checks if an error is a timeout
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsConnectionError checks if an error is a connection error
func IsConnectionError(err error) bool {
	return errors.Is(err, ErrConnectionFailed)
}

// IsLockError checks if an error is a lock error
func IsLockError(err error) bool {
	return errors.Is(err, ErrLockAcquisition) || errors.Is(err, ErrLockNotHeld)
}
