package registry

import (
	"context"
	"io"
	"log"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// startRegistry runs an in-memory OCI registry and returns its host.
func startRegistry(t *testing.T) string {
	t.Helper()
	s := httptest.NewServer(ggcrregistry.New(ggcrregistry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(s.Close)
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func TestCopy(t *testing.T) {
	host := startRegistry(t)
	srcPath := host + "/dev/my-app"
	dstPath := host + "/prod/my-app"

	// Seed the source repository with a random image.
	img, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}
	tag, err := name.NewTag(srcPath + ":seed")
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(tag, img); err != nil {
		t.Fatal(err)
	}

	srcRef := srcPath + "@" + digest.String()
	copier := &Copier{keychain: authn.DefaultKeychain}
	ctx := context.Background()

	copied, err := copier.Copy(ctx, srcRef, dstPath)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if !copied {
		t.Error("first Copy: want copied=true")
	}

	// Destination must contain the identical digest.
	dstRef, err := name.NewDigest(dstPath + "@" + digest.String())
	if err != nil {
		t.Fatal(err)
	}
	desc, err := remote.Head(dstRef)
	if err != nil {
		t.Fatalf("destination missing after copy: %v", err)
	}
	if desc.Digest.String() != digest.String() {
		t.Errorf("digest mismatch: %s != %s", desc.Digest, digest)
	}

	// Second copy is a no-op.
	copied, err = copier.Copy(ctx, srcRef, dstPath)
	if err != nil {
		t.Fatalf("second Copy: %v", err)
	}
	if copied {
		t.Error("second Copy: want copied=false (idempotent)")
	}
}

func TestCopyRejectsTagReference(t *testing.T) {
	copier := &Copier{keychain: authn.DefaultKeychain}
	_, err := copier.Copy(context.Background(), "gcr.io/p/app:latest", "gcr.io/p2/app")
	if err == nil || !strings.Contains(err.Error(), "digest-pinned") {
		t.Errorf("want digest-pinned error, got %v", err)
	}
}

func TestCopyMissingSource(t *testing.T) {
	host := startRegistry(t)
	srcRef := host + "/dev/ghost@sha256:" + strings.Repeat("a", 64)
	copier := &Copier{keychain: authn.DefaultKeychain}
	if _, err := copier.Copy(context.Background(), srcRef, host+"/prod/ghost"); err == nil {
		t.Error("missing source: want error")
	}
}
