package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Config struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
}

type S3Store struct {
	cfg    S3Config
	client *s3.Client
}

func NewS3Store(cfg S3Config) (*S3Store, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.ForcePathStyle
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})
	return &S3Store{cfg: cfg, client: client}, nil
}

func (s *S3Store) objectKey(digest string) (string, error) {
	hexPart, err := normalizeDigest(digest)
	if err != nil {
		return "", err
	}
	return "sha256/" + hexPart[:2] + "/" + hexPart[2:4] + "/" + hexPart, nil
}

func (s *S3Store) Put(ctx context.Context, digest string, r io.Reader, size int64) (BlobInfo, error) {
	tmp, err := os.CreateTemp("", "shipyard-s3-*")
	if err != nil {
		return BlobInfo{}, fmt.Errorf("create temp blob: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), r)
	if err != nil {
		return BlobInfo{}, fmt.Errorf("write blob: %w", err)
	}
	if size >= 0 && written != size {
		return BlobInfo{}, fmt.Errorf("blob size mismatch: expected %d got %d", size, written)
	}

	computed := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if digest != "" && digest != computed {
		return BlobInfo{}, fmt.Errorf("digest mismatch: expected %s got %s", digest, computed)
	}
	finalKey, err := s.objectKey(computed)
	if err != nil {
		return BlobInfo{}, err
	}
	if _, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(finalKey)}); err == nil {
		return BlobInfo{Digest: computed, Size: written}, nil
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return BlobInfo{}, err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.cfg.Bucket),
		Key:           aws.String(finalKey),
		Body:          tmp,
		ContentLength: aws.Int64(written),
	})
	if err != nil {
		return BlobInfo{}, fmt.Errorf("s3 put: %w", err)
	}
	return BlobInfo{Digest: computed, Size: written}, nil
}

func (s *S3Store) Get(ctx context.Context, digest string) (io.ReadCloser, BlobInfo, error) {
	key, err := s.objectKey(digest)
	if err != nil {
		return nil, BlobInfo{}, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, BlobInfo{}, fmt.Errorf("s3 get: %w", err)
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, BlobInfo{Digest: digest, Size: size}, nil
}

func (s *S3Store) Stat(ctx context.Context, digest string) (BlobInfo, error) {
	key, err := s.objectKey(digest)
	if err != nil {
		return BlobInfo{}, err
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key)})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "NotFound") || strings.Contains(msg, "404") || strings.Contains(msg, "NoSuchKey") {
			return BlobInfo{}, fmt.Errorf("%w", os.ErrNotExist)
		}
		return BlobInfo{}, err
	}
	var size int64
	if head.ContentLength != nil {
		size = *head.ContentLength
	}
	return BlobInfo{Digest: digest, Size: size}, nil
}

func (s *S3Store) Exists(ctx context.Context, digest string) (bool, error) {
	key, err := s.objectKey(digest)
	if err != nil {
		return false, err
	}
	_, err = s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "NotFound") || strings.Contains(msg, "404") || strings.Contains(msg, "NoSuchKey") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3Store) Delete(ctx context.Context, digest string) error {
	key, err := s.objectKey(digest)
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	return err
}
