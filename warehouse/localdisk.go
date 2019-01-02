package warehouse

import (
	"context"
	"fmt"
	"github.com/fullstorydev/hauser/config"
	"github.com/nishanths/fullstory"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

type LocalDisk struct {
	conf *config.Config
}

const (
	timestampFile string = ".sync.hs"
)

var _ Warehouse = (*LocalDisk)(nil)

func NewLocalDisk(c *config.Config) *LocalDisk {
	if c.Local.UseStartTime {
		filename := filepath.Join(c.Local.SaveDir, timestampFile)
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			os.Remove(filename)
		}
	}

	return &LocalDisk{
		conf: c,
	}
}

// Currently returns the names of all the fields exported by BundleFields
func (w *LocalDisk) GetExportTableColumns() []string {
	allFields := BundleFields()
	fieldNames := make([]string, 0, len(allFields))
	for name := range allFields {
		fieldNames = append(fieldNames, name)
	}
	return fieldNames
}

// Reads the last sync date from file
func (w *LocalDisk) LastSyncPoint() (time.Time, error) {
	t := beginningOfTime
	if w.conf.Local.UseStartTime {
		t = w.conf.Local.StartTime
	}
	if _, err := os.Stat(w.conf.Local.SaveDir); os.IsNotExist(err) {
		panic(err)
	}
	filename := filepath.Join(w.conf.Local.SaveDir, timestampFile)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return t, nil
	}
	timebytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return t, err
	}
	synctime, err := time.Parse(time.RFC3339, string(timebytes))
	return synctime, err
}

// Writes the current time to timestamp file
func (w *LocalDisk) SaveSyncPoints(bundles ...fullstory.ExportMeta) error {
	t := time.Now().UTC().Format(time.RFC3339)
	if _, err := os.Stat(w.conf.Local.SaveDir); os.IsNotExist(err) {
		panic(err)
	}
	filename := filepath.Join(w.conf.Local.SaveDir, timestampFile)
	timedata := []byte(t)
	return ioutil.WriteFile(filename, timedata, 0644)
}

func (w *LocalDisk) LoadToWarehouse(objName string, bundles ...fullstory.ExportMeta) error {
	//Do nothing, as that method is called during the current workflow
	return nil
}

func (w *LocalDisk) EnsureCompatibleExportTable() error {
	// noop, at least for now. It's ok for JSON files to have different schemas, and ensuring
	// that previously downloaded CSV files have a consistent schema is non-trivial to solve efficiently.
	return nil
}

// Defers to common implementation
func (w *LocalDisk) ValueToString(val interface{}, isTime bool) string {
	return ValueToString(val, isTime)
}

func (w *LocalDisk) UploadFile(filename string) (string, error) {
	baseFile := filepath.Base(filename)
	copiedFileName := filepath.Join(w.conf.Local.SaveDir, baseFile)
	srcFile, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(copiedFileName)
	if err != nil {
		return "", err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return "", err
	}
	return copiedFileName, nil
}

// This method should do nothing for local disk
func (w *LocalDisk) DeleteFile(objName string) {}

func (w *LocalDisk) GetUploadFailedMsg(filename string, err error) string {
	return fmt.Sprintf("Failed to copy file %s to local dir %s: %s", filename, w.conf.Local.SaveDir, err)
}

func (w *LocalDisk) IsUploadOnly() bool {
	//We need to return false, otherwise sync point will never be saved
	return false
}
