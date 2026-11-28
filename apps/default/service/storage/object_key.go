package storage

import (
	"errors"
	"fmt"
	"path"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/pitabwire/util"
)

const (
	minHashLen = 3
	maxHashLen = 255

	// plainObjectName names content-addressed (unencrypted) objects.
	plainObjectName = "file"
)

// ContentObjectKey returns the bucket-relative key to store meta's content
// under. Keys never depend on the pod's filesystem layout; the key chosen at
// upload is recorded on the row and used for every later read and delete.
//
// Unencrypted content is content-addressed (<h0>/<h1>/<h2..>/file): the key
// is deterministic, so identical uploads share one object. Encrypted content
// is sealed with a per-upload data key, so its ciphertext is unique to that
// upload and must never be shared or overwritten; it gets a fresh key under
// the same hash prefix (<h0>/<h1>/<h2..>/<unique id>) on every call. That
// also keeps a reused media ID or a stale object from an earlier upload from
// ever being mistaken for this upload's ciphertext.
func ContentObjectKey(meta *types.MediaMetadata) (string, error) {
	if meta == nil {
		return "", errors.New("object key: nil media metadata")
	}
	hash := string(meta.Base64Hash)
	if err := validateKeySegment(hash, minHashLen, maxHashLen); err != nil {
		return "", fmt.Errorf("object key: invalid hash %q: %w", hash, err)
	}

	name := plainObjectName
	if meta.Encryption != nil {
		name = util.IDString()
		if err := validateKeySegment(name, 1, maxHashLen); err != nil {
			return "", fmt.Errorf("object key: invalid object id %q: %w", name, err)
		}
	}

	return path.Join(hash[0:1], hash[1:2], hash[2:], name), nil
}

// BucketSelector picks the bucket for a visibility; every Provider is one.
type BucketSelector interface {
	GetBucket(isPublic bool) string
}

// ResolveObjectLocation returns the bucket and key holding meta's content.
//
// Rows record the bucket and key they were written to. Rows written before
// that (empty BucketName/ObjectKey) resolve to the legacy layout: the bucket
// chosen by visibility and the absolute hash path under legacyBasePath, which
// is the key those objects were uploaded under.
func ResolveObjectLocation(provider BucketSelector, meta *types.MediaMetadata, legacyBasePath config.Path) (string, string, error) {
	if meta == nil {
		return "", "", errors.New("object location: nil media metadata")
	}

	bucket := meta.BucketName
	if bucket == "" {
		bucket = provider.GetBucket(meta.IsPublic)
	}

	key := meta.ObjectKey
	if key == "" {
		legacyKey, err := utils.GetPathFromBase64Hash(meta.Base64Hash, legacyBasePath)
		if err != nil {
			return "", "", fmt.Errorf("object location: legacy key: %w", err)
		}
		key = legacyKey
	}

	return bucket, key, nil
}

// validateKeySegment accepts only the base64url alphabet (plus '='), what
// hashes and generated IDs are made of, so a segment can never introduce a
// path separator or a dot segment into an object key.
func validateKeySegment(s string, minLen, maxLen int) error {
	if len(s) < minLen || len(s) > maxLen {
		return fmt.Errorf("length %d outside [%d,%d]", len(s), minLen, maxLen)
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '=':
		default:
			return fmt.Errorf("unsupported character %q", r)
		}
	}
	return nil
}
