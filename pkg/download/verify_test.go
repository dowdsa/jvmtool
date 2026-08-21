package download

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySHA256RejectsEmptyExpected(t *testing.T) {
	f := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := VerifySHA256(f, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("VerifySHA256 should return false for empty expected, not true")
	}
}

func TestVerifySHA512RejectsEmptyExpected(t *testing.T) {
	f := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := VerifySHA512(f, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("VerifySHA512 should return false for empty expected, not true")
	}
}
