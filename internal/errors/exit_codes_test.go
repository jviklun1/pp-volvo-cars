package errors

import (
	stderrors "errors"
	"fmt"
	"testing"
)

func TestCodeOf(t *testing.T) {
	base := stderrors.New("failure")
	tests := []struct {
		name string
		err  error
		want Code
	}{
		{name: "success", err: nil, want: CodeOK},
		{name: "internal fallback", err: base, want: CodeInternal},
		{name: "authentication", err: WithCode(CodeAuthentication, base), want: CodeAuthentication},
		{name: "not found", err: WithCode(CodeNotFound, base), want: CodeNotFound},
		{name: "invalid arguments", err: WithCode(CodeInvalidArguments, base), want: CodeInvalidArguments},
		{name: "insufficient evidence", err: WithCode(CodeInsufficientEvidence, base), want: CodeInsufficientEvidence},
		{name: "rate limited", err: WithCode(CodeRateLimited, base), want: CodeRateLimited},
		{name: "wrapped typed error", err: fmt.Errorf("outer: %w", WithCode(CodeNotFound, base)), want: CodeNotFound},
		{name: "wrapped rate-limit sentinel", err: fmt.Errorf("outer: %w", ErrRateLimited), want: CodeRateLimited},
		{name: "authentication sentinel", err: ErrAuthentication, want: CodeAuthentication},
		{name: "success code cannot mask failure", err: WithCode(CodeOK, base), want: CodeInternal},
		{name: "invalid typed code", err: WithCode(Code(99), base), want: CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CodeOf(tt.err); got != tt.want {
				t.Fatalf("CodeOf() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExitErrorUnwrap(t *testing.T) {
	base := stderrors.New("failure")
	err := WithCode(CodeAuthentication, base)
	if !stderrors.Is(err, base) {
		t.Fatal("ExitError must preserve its underlying cause")
	}
}
