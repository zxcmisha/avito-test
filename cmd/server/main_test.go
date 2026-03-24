package main

import (
	"os"
	"testing"
)

func TestGetenv(t *testing.T) {
	const key = "UNIT_TEST_ENV_KEY"
	_ = os.Unsetenv(key)
	if v := getenv(key, "fallback"); v != "fallback" {
		t.Fatalf("expected fallback, got %s", v)
	}
	if err := os.Setenv(key, "value"); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv(key)
	if v := getenv(key, "fallback"); v != "value" {
		t.Fatalf("expected value, got %s", v)
	}
}
