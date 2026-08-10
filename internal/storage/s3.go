package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
	hasher := sha256.New()
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		written, err := io.Copy(io.MultiWriter(pw, hasher), r)
		if err != nil {
			_ = pw.CloseWithError(err)
			errCh <- err
			return
		}
		if size >= 0 && written != size {
			err = fmt.Errorf("blob size mismatch: expected %d got %d", size, written)
			_ = pw.CloseWithError(err)
			errCh <- err
			return
		}
		errCh <- nil
	}()

	tmp := make([]byte, 16)
	_, _ = rand.Read(tmp)
	tmpKey := "uploads/" + hex.EncodeToString(tmp)

	_, putErr := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(tmpKey),
		Body:   pr,
	})
	copyErr := <-errCh
	if putErr != nil {
		return BlobInfo{}, fmt.Errorf("s3 put: %w", putErr)
	}
	if copyErr != nil {
		_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(tmpKey)})
		return BlobInfo{}, copyErr
	}

	computed := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if digest != "" && digest != computed {
		_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(tmpKey)})
		return BlobInfo{}, fmt.Errorf("digest mismatch: expected %s got %s", digest, computed)
	}
	finalKey, err := s.objectKey(computed)
	if err != nil {
		return BlobInfo{}, err
	}
	_, err = s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.cfg.Bucket),
		CopySource: aws.String(s.cfg.Bucket + "/" + tmpKey),
		Key:        aws.String(finalKey),
	})
	if err != nil {
		return BlobInfo{}, fmt.Errorf("s3 finalize: %w", err)
	}
	_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(tmpKey)})

	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(finalKey)})
	if err != nil {
		return BlobInfo{Digest: computed}, nil
	}
	var outSize int64
	if head.ContentLength != nil {
		outSize = *head.ContentLength
	}
	return BlobInfo{Digest: computed, Size: outSize}, nil
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
