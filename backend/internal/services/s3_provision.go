package services

import (
	"context"
	"errors"
	"fmt"

	"Noooste/garage-ui/internal/models"
	logpkg "Noooste/garage-ui/pkg/logger"
	"Noooste/garage-ui/pkg/utils"

	"github.com/minio/minio-go/v7/pkg/credentials"
)

// defaultUIKeyName is used when garage.ui_key_name is empty.
const defaultUIKeyName = "garage-ui"

func (s *S3Service) uiKeyName() string {
	if s.config.UIKeyName != "" {
		return s.config.UIKeyName
	}
	return defaultUIKeyName
}

// provisionBucketAccess is the opt-in fallback of getBucketCredentials
// (garage.auto_provision_bucket_access). It makes sure Garage UI's own key
// exists, grants it read+write (never owner) on the bucket, and returns that
// key's credentials.
//
// It is only reached from S3 object operations, i.e. after the HTTP layer has
// authenticated and authorised the request, and only for buckets that exist
// (the bucket lookup in getBucketCredentials fails first otherwise).
//
// Calls are serialised: concurrent requests for a bucket without keys queue up
// and the later ones find the credentials already cached.
func (s *S3Service) provisionBucketAccess(ctx context.Context, bucketName string, op Operation) (*credentials.Credentials, error) {
	s.provisionMu.Lock()
	defer s.provisionMu.Unlock()

	// A request that held the lock before us may already have done the work.
	if cached := s.cachedCredentials(bucketName, op); cached != nil {
		return cached, nil
	}

	log := logpkg.FromCtx(ctx)

	bucketInfo, err := s.adminService.GetBucketInfoByAlias(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket info: %w", err)
	}
	if creds := s.credentialsFromBucketKeys(ctx, bucketName, bucketInfo, op); creds != nil {
		return creds, nil
	}

	keyID, err := s.ensureUIKey(ctx)
	if err != nil {
		return nil, err
	}

	// Allow is additive, so this also tops up a UI key that exists on the
	// bucket with fewer permissions. Never owner.
	_, err = s.adminService.AllowBucketKey(ctx, models.BucketKeyPermRequest{
		BucketID:    bucketInfo.ID,
		AccessKeyID: keyID,
		Permissions: models.BucketKeyPermission{Read: true, Write: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to grant key %s read/write on bucket %s: %w", logpkg.RedactKey(keyID), bucketName, err)
	}
	log.Info().
		Str("component", "s3").
		Str("bucket", bucketName).
		Str("access_key_id", logpkg.RedactKey(keyID)).
		Msg("auto-provisioned bucket access: granted read/write to the Garage UI key")

	keyDetails, err := s.adminService.GetKeyInfo(ctx, keyID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch key %s: %w", logpkg.RedactKey(keyID), err)
	}
	if keyDetails.SecretAccessKey == nil {
		return nil, fmt.Errorf("garage returned no secret for key %s", logpkg.RedactKey(keyID))
	}

	creds := credentials.NewStaticV4(keyDetails.AccessKeyID, *keyDetails.SecretAccessKey, "")
	setKeyInCache(bucketName, models.BucketKeyPermission{Read: true, Write: true}, creds)
	return creds, nil
}

// ensureUIKey returns the ID of the key named garage.ui_key_name, creating it
// when it does not exist. The key is created without any global permission
// (in particular without create-bucket). If several non-expired keys share the
// name, the one with the smallest ID is used so the choice is stable.
// Must be called with provisionMu held.
func (s *S3Service) ensureUIKey(ctx context.Context) (string, error) {
	name := s.uiKeyName()

	keys, err := s.adminService.ListKeys(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list keys: %w", err)
	}
	found := ""
	for _, k := range keys {
		if k.Name == name && !k.Expired && (found == "" || k.ID < found) {
			found = k.ID
		}
	}
	if found != "" {
		return found, nil
	}

	created, err := s.adminService.CreateKey(ctx, models.CreateKeyRequest{
		Name:         &name,
		NeverExpires: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create key %q: %w", name, err)
	}
	if created == nil || created.AccessKeyID == "" {
		return "", errors.New("garage returned no key id for the created key")
	}
	logpkg.FromCtx(ctx).Info().
		Str("component", "s3").
		Str("key_name", name).
		Str("access_key_id", logpkg.RedactKey(created.AccessKeyID)).
		Msg("auto-provisioned bucket access: created the Garage UI key")
	return created.AccessKeyID, nil
}

// cachedCredentials returns cached credentials for the bucket/op, or nil.
func (s *S3Service) cachedCredentials(bucketName string, op Operation) *credentials.Credentials {
	if c, ok := utils.GlobalCache.Get(fmt.Sprintf("key:%s:%d", bucketName, op)).(*credentials.Credentials); ok {
		return c
	}
	return nil
}
