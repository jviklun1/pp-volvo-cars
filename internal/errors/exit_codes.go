// Package errors defines stable process exit behavior for CLI callers.
package errors

import (
	stderrors "errors"
	"fmt"
)

// Code is a stable process exit code in the CLI contract.
type Code int

const (
	CodeOK                   Code = 0
	CodeInternal             Code = 1
	CodeAuthentication       Code = 2
	CodeNotFound             Code = 3
	CodeInvalidArguments     Code = 4
	CodeInsufficientEvidence Code = 5
	CodeRateLimited          Code = 7
)

var (
	ErrRateLimited    = stderrors.New("rate limited")
	ErrAuthentication = stderrors.New("authentication failed")
)

// Valid reports whether the code is part of the public CLI contract.
func (code Code) Valid() bool {
	switch code {
	case CodeOK,
		CodeInternal,
		CodeAuthentication,
		CodeNotFound,
		CodeInvalidArguments,
		CodeInsufficientEvidence,
		CodeRateLimited:
		return true
	default:
		return false
	}
}

// ExitError carries a stable exit code while retaining the underlying error.
type ExitError struct {
	Code Code
	Err  error
}

func (err *ExitError) Error() string {
	if err == nil {
		return "<nil>"
	}
	if err.Err == nil {
		return fmt.Sprintf("command failed with exit code %d", err.Code)
	}
	return err.Err.Error()
}

func (err *ExitError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

// WithCode attaches a stable exit code to an error.
func WithCode(code Code, err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: code, Err: err}
}

// CodeOf maps nil, typed, and unclassified failures to process exit codes.
func CodeOf(err error) Code {
	if err == nil {
		return CodeOK
	}
	var exitErr *ExitError
	if stderrors.As(err, &exitErr) && exitErr.Code.Valid() && exitErr.Code != CodeOK {
		return exitErr.Code
	}
	if stderrors.Is(err, ErrRateLimited) {
		return CodeRateLimited
	}
	if stderrors.Is(err, ErrAuthentication) {
		return CodeAuthentication
	}
	return CodeInternal
}
