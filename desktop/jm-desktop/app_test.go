package main

import (
	"context"
	"testing"
)

func TestAppMethods(t *testing.T) {
	a := NewApp()
	a.startup(context.Background())

	// Root should be non-empty
	if a.Root() == "" {
		t.Fatal("Root() should not be empty")
	}

	// List should not panic for both kinds
	for _, kind := range []string{"jdk", "maven"} {
		_ = a.List(kind)
		_ = a.Current(kind)
	}
}
