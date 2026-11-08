package client

import (
	"errors"
	"strings"
	"testing"
)

func TestGatewayClosedError(t *testing.T) {
	err := &GatewayClosedError{Reason: 8}
	if !strings.Contains(err.Error(), "原因码 8") {
		t.Fatalf("error %q", err.Error())
	}
	var target *GatewayClosedError
	if !errors.As(err, &target) || target.Reason != 8 {
		t.Fatal("errors.As did not find the reason")
	}
}
