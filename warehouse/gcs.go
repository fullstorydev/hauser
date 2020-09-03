package warehouse

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
)

type GCSStorage struct {
	config    *config.GCSConfig
	gcsClient *storage.Client
}

var _ Storage = (*GCSStorage)(nil)

func NewGCSStorage(conf *config.GCSConfig, gcsClient *storage.Client) *GCSStorage {
	return &GCSStorage{
		config:    conf,
		gcsClient: gcsClient,
	}
}

func (g *GCSStorage) LastSyncPoint(ctx context.Context) (time.Time, error) {
	return StorageMixin{g}.LastSyncPoint(ctx)
}

func (g *GCSStorage) SaveSyncPoints(ctx context.Context, bundles ...client.ExportMeta) error {
	return StorageMixin{g}.SaveSyncPoints(ctx, bundles...)
}

func (g *GCSStorage) SaveFile(ctx context.Context, name string, reader io.Reader) (string, error) {
	w := g.bucket().Object(name).NewWriter(ctx)
	if _, err := io.Copy(w, reader); err != nil {
		return "", fmt.Errorf("failed to save file to GCS: %s", err)
	}
	return g.GetFileReference(name), w.Close()
}

func (g *GCSStorage) ReadFile(ctx context.Context, name string) (io.Reader, error) {
	reader, err := g.bucket().Object(name).NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, ErrFileNotFound
	}
	return reader, err
}

func (g *GCSStorage) DeleteFile(ctx context.Context, name string) error {
	if err := g.bucket().Object(name).Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete %s from GCS bucket %s: %s", name, g.config.Bucket, err)
	}
	return nil
}

func (g *GCSStorage) GetFileReference(name string) string {
	return fmt.Sprintf("gs://%s/%s", g.config.Bucket, name)
}

func (g *GCSStorage) bucket() *storage.BucketHandle {
	return g.gcsClient.Bucket(g.config.Bucket)
}
