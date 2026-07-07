package manifest

import (
	"fmt"
	"strings"
)

const digestSeparator = "@sha256:"

// IsDigestPinned reports whether image references a sha256 digest.
func IsDigestPinned(image string) bool {
	_, _, err := SplitDigest(image)
	return err == nil
}

// SplitDigest splits "REPO/IMAGE@sha256:HEX" into the image path and the
// digest ("sha256:HEX").
func SplitDigest(image string) (path, digest string, err error) {
	i := strings.LastIndex(image, digestSeparator)
	if i < 0 {
		return "", "", fmt.Errorf("image %q is not digest-pinned (tags are not allowed; use IMAGE@sha256:...)", image)
	}
	path, digest = image[:i], image[i+1:]
	hex := strings.TrimPrefix(digest, "sha256:")
	if path == "" || len(hex) != 64 {
		return "", "", fmt.Errorf("image %q has a malformed digest reference", image)
	}
	return path, digest, nil
}

// WithDigest builds a digest-pinned reference from an image path and a
// "sha256:HEX" digest.
func WithDigest(path, digest string) string {
	return path + "@" + digest
}

// ShortDigest returns a truncated digest ("sha256:HEX" -> first 12 hex
// chars) for display and commit messages.
func ShortDigest(digest string) string {
	hex := strings.TrimPrefix(digest, "sha256:")
	if len(hex) > 12 {
		hex = hex[:12]
	}
	return hex
}
