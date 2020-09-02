package warehouse

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/fullstorydev/hauser/config"
)

type LocalDisk struct {
	StorageMixin
	conf *config.LocalConfig
}

var _ Storage = (*LocalDisk)(nil)

func NewLocalDisk(c *config.LocalConfig) *LocalDisk {
	if _, err := os.Stat(c.SaveDir); os.IsNotExist(err) {
		errorMessage := fmt.Sprintf("Cannot find folder %s, make sure it exists", c.SaveDir)
		log.Fatalf(errorMessage)
	}
	if c.UseStartTime && c.StartTime.IsZero() {
		log.Fatalf("Asked to use Start Time, but it is not specified")
	}

	if c.UseStartTime {
		filename := filepath.Join(c.SaveDir, timestampFile)
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			os.Remove(filename)
		}
	}

	return &LocalDisk{
		conf: c,
	}
}

func (w *LocalDisk) SaveFile(_ context.Context, name string, reader io.Reader) error {
	filename := filepath.Join(w.conf.SaveDir, name)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = io.Copy(f, reader); err != nil {
		return err
	}
	return nil
}

// DeleteFile should do nothing for local disk
func (w *LocalDisk) DeleteFile(_ context.Context, name string) error {
	filename := filepath.Join(w.conf.SaveDir, name)
	return os.Remove(filename)
}

func (w *LocalDisk) ReadFile(_ context.Context, name string) (io.Reader, error) {
	filename := filepath.Join(w.conf.SaveDir, timestampFile)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}
	return os.Open(filename)
}
