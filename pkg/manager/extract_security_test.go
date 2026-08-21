package manager

import "testing"

func TestMaxExtractSizeConstant(t *testing.T) {
	// Verify the constant is 2 GB as documented.
	if maxExtractSize != 2<<30 {
		t.Errorf("maxExtractSize = %d, want %d", maxExtractSize, 2<<30)
	}
}
