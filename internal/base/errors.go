package base

import (
	"context"
	"errors"
	"fmt"
)

/* ------------------ Sentinel Errors ------------------ */

// These are stable, comparable errors.
// NEVER wrap these directly; always wrap via CacheError.

var (
	ErrKeyEmpty      = errors.New("key is empty")
	ErrCacheMiss     = errors.New("cache miss")
	ErrInvalidConfig = errors.New("invalid config")

	ErrSerialize   = errors.New("serialization failed")
	ErrDeserialize = errors.New("deserialization failed")

	ErrConnection = errors.New("connection failed")

	ErrLockAcquire = errors.New("lock acquisition failed")
	ErrLockNotHeld = errors.New("lock not held")
)

/* ------------------ Operation ------------------ */

type Op string

const (
	OpGet             Op = "get"
	OpSet             Op = "set"
	OpDelete          Op = "delete"
	OpExists          Op = "exists"
	OpClear           Op = "clear"
	OpLen             Op = "len"
	OpGetOrSet        Op = "get_or_set"
	OpGetOrSetLocked  Op = "get_or_set_locked"
	OpGetManyPipeline Op = "get_many_pipeline"
	OpSetManyPipeline Op = "set_many_pipeline"
	OpDeleteByPrefix  Op = "delete_by_prefix"
	OpPing            Op = "ping"
	OpLock            Op = "lock"
	OpUnlock          Op = "unlock"
	OpTryLock         Op = "try_lock"
	OpInit            Op = "init"
)

/* ------------------ CacheError ------------------ */

// CacheError wraps an underlying error with operation context.
// It MUST always wrap a sentinel or foreign error.

type CacheError struct {
	Op  Op
	Key string
	Err error
}

func (e *CacheError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("%s [%s]: %v", e.Op, e.Key, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *CacheError) Unwrap() error {
	return e.Err
}

/* ------------------ Constructors ------------------ */

func WrapError(op Op, err error, key string) error {
	if err == nil {
		return nil
	}
	if op == "" {
		panic("cache error: empty op")
	}
	return &CacheError{
		Op:  op,
		Err: err,
		Key: key,
	}
}

/* ------------------ Classification Helpers ------------------ */

func IsCacheMiss(err error) bool {
	return errors.Is(err, ErrCacheMiss)
}

func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

func IsConnectionError(err error) bool {
	return errors.Is(err, ErrConnection)
}

func IsLockError(err error) bool {
	return errors.Is(err, ErrLockAcquire) ||
		errors.Is(err, ErrLockNotHeld)
}

func IsSerializationError(err error) bool {
	return errors.Is(err, ErrSerialize) ||
		errors.Is(err, ErrDeserialize)
}

/* ------------------ Retryability ------------------ */

func IsRetryable(err error) bool {
	switch {
	case IsContextError(err):
		return false
	case IsCacheMiss(err):
		return false
	case IsSerializationError(err):
		return false
	default:
		return IsConnectionError(err) || IsLockError(err)
	}
}
