package version

import (
	"context"

	"jm/pkg/config"
)

// OpenJDKSource wraps the Adoptium API to provide OpenJDK builds.
//
// Oracle's jdk.java.net doesn't offer a stable API — download URLs contain
// per-release hashes that change unpredictably. Instead, we use the Adoptium
// API which serves the same OpenJDK binaries (Temurin is Eclipse's build of
// OpenJDK) with proper SHA256 checksums, stable URLs, and mirror support.
type OpenJDKSource struct {
	inner *JDKSource
}

func NewOpenJDKSource() *OpenJDKSource {
	return &OpenJDKSource{
		inner: &JDKSource{
			Arch:      hostArch(),
			OS:        hostOS(),
			ImageType: "jdk",
			Client:    config.HTTPClient(),
		},
	}
}

// List delegates to the Adoptium-backed JDKSource.
func (s *OpenJDKSource) List(ctx context.Context, query string, limit int) ([]string, error) {
	return s.inner.List(ctx, query, limit)
}

// Resolve delegates to the Adoptium-backed JDKSource.
func (s *OpenJDKSource) Resolve(ctx context.Context, version string) (*Artifact, error) {
	return s.inner.Resolve(ctx, version)
}

// Ensure OpenJDKSource satisfies the Source interface.
var _ Source = (*OpenJDKSource)(nil)
