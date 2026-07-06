package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// keychain resolves Artifact Registry credentials via ADC and falls back
// to the local docker config.
var keychain = authn.NewMultiKeychain(google.Keychain, authn.DefaultKeychain)

// Copier copies digest-pinned images between repositories.
type Copier struct {
	keychain authn.Keychain
}

// NewCopier returns a Copier authenticating with ADC / docker config.
func NewCopier() *Copier {
	return &Copier{keychain: keychain}
}

// Copy ensures the image at srcRef ("REPO/IMAGE@sha256:HEX") exists in the
// dstPath repository ("REPO/IMAGE", no tag or digest) under the same digest.
// It returns whether a copy was performed; false means the digest was
// already present (the call is idempotent). After copying, the destination
// digest is verified to match the source.
func (c *Copier) Copy(ctx context.Context, srcRef, dstPath string) (copied bool, err error) {
	src, err := name.NewDigest(srcRef)
	if err != nil {
		return false, fmt.Errorf("source image must be digest-pinned: %w", err)
	}
	dst, err := name.NewDigest(dstPath + "@" + src.DigestStr())
	if err != nil {
		return false, fmt.Errorf("invalid copy destination %q: %w", dstPath, err)
	}

	exists, err := c.exists(ctx, dst)
	if err != nil {
		return false, fmt.Errorf("checking %s: %w", dst, err)
	}
	if exists {
		return false, nil
	}

	opts := []crane.Option{
		crane.WithContext(ctx),
		crane.WithAuthFromKeychain(c.keychain),
	}
	if err := crane.Copy(src.String(), dst.String(), opts...); err != nil {
		return false, fmt.Errorf("copying %s to %s: %w", src, dstPath, err)
	}

	// The destination is addressed by digest, so a successful HEAD proves
	// bit-identity with the source.
	verified, err := c.exists(ctx, dst)
	if err != nil {
		return false, fmt.Errorf("verifying copy of %s: %w", dst, err)
	}
	if !verified {
		return false, fmt.Errorf("copy of %s to %s did not verify", src, dstPath)
	}
	return true, nil
}

func (c *Copier) exists(ctx context.Context, ref name.Digest) (bool, error) {
	_, err := remote.Head(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(c.keychain))
	if err == nil {
		return true, nil
	}
	var terr *transport.Error
	if errors.As(err, &terr) && terr.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, err
}
