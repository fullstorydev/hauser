package warehouse

import (
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"github.com/fullstorydev/hauser/config"
)

type GCSStorage struct {
	StorageMixin
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

func (g *GCSStorage) SaveFile(ctx context.Context, name string, reader io.Reader) error {
	w := g.bucket().Object(name).NewWriter(ctx)
	if _, err := io.Copy(w, reader); err != nil {
		return fmt.Errorf("failed to save file to GCS: %s", err)
	}
	return w.Close()
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
