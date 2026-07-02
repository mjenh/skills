package tapo

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrorsAreDistinct(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrAuth", ErrAuth},
		{"ErrTimeout", ErrTimeout},
		{"ErrUnsupportedModel", ErrUnsupportedModel},
		{"ErrHandshake", ErrHandshake},
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && a.err == b.err {
				t.Errorf("%s and %s must be distinct sentinel values", a.name, b.name)
			}
		}
	}
}

func TestSentinelErrorsWrapping(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrAuth", ErrAuth},
		{"ErrTimeout", ErrTimeout},
		{"ErrUnsupportedModel", ErrUnsupportedModel},
		{"ErrHandshake", ErrHandshake},
	}

	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			wrapped := fmt.Errorf("context: %w", s.err)
			if !errors.Is(wrapped, s.err) {
				t.Errorf("errors.Is(wrapped, %s) should be true", s.name)
			}
		})
	}
}

func TestSentinelErrorsNoCrossMatch(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrAuth", ErrAuth},
		{"ErrTimeout", ErrTimeout},
		{"ErrUnsupportedModel", ErrUnsupportedModel},
		{"ErrHandshake", ErrHandshake},
	}

	for i, a := range sentinels {
		wrapped := fmt.Errorf("context: %w", a.err)
		for j, b := range sentinels {
			if i != j && errors.Is(wrapped, b.err) {
				t.Errorf("errors.Is(wrapped %s, %s) should be false", a.name, b.name)
			}
		}
	}
}
