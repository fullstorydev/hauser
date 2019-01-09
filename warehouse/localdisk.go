package warehouse

import (
	"fmt"
	"github.com/fullstorydev/hauser/config"
	"github.com/nishanths/fullstory"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

type LocalDisk struct {
	conf *config.Config
}

const (
	timestampFile string = ".sync.hauser"
)

var _ Warehouse = (*LocalDisk)(nil)

func NewLocalDisk(c *config.Config) *LocalDisk {
	if _, err := os.Stat(c.Local.SaveDir); os.IsNotExist(err) {
		errorMessage := fmt.Sprintf("Cannot find folder %s, make sure it exists", c.Local.SaveDir)
		log.Fatalf(errorMessage)
	}
	if c.Local.UseStartTime && c.Local.StartTime.IsZero() {
		log.Fatalf("Asked to use Start Time, but it is not specified")
	}

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

// GetExportTableColumns currently returns the names of all the fields exported by BundleFields
func (w *LocalDisk) GetExportTableColumns() []string {
	allFields := BundleFields()
	fieldNames := make([]string, 0, len(allFields))
	for name := range allFields {
		fieldNames = append(fieldNames, name)
	}
	return fieldNames
}

// LastSyncPoint reads the last sync date from file
func (w *LocalDisk) LastSyncPoint() (time.Time, error) {
	t := beginningOfTime
	if w.conf.Local.UseStartTime {
		t = w.conf.Local.StartTime
	}
	filename := filepath.Join(w.conf.Local.SaveDir, timestampFile)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return t, nil
	}
	timebytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return t, err
	}
	return time.Parse(time.RFC3339, string(timebytes))
}

// SaveSyncPoints writes the current time to timestamp file
func (w *LocalDisk) SaveSyncPoints(bundles ...fullstory.ExportMeta) error {
	if len(bundles) == 0 {
		panic("Zero-length bundle list passed to SaveSyncPoints")
	}
	t := bundles[len(bundles)-1].Stop.UTC().Format(time.RFC3339)
	if _, err := os.Stat(w.conf.Local.SaveDir); os.IsNotExist(err) {
		log.Fatalf(err.Error())
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

// ValueToString defers to common implementation
func (w *LocalDisk) ValueToString(val interface{}, isTime bool) string {
	return valueToString(val, isTime)
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
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return "", err
	}
	return copiedFileName, nil
}

// DeleteFile should do nothing for local disk
func (w *LocalDisk) DeleteFile(objName string) {}

func (w *LocalDisk) GetUploadFailedMsg(filename string, err error) string {
	return fmt.Sprintf("Failed to copy file %s to local dir %s: %s", filename, w.conf.Local.SaveDir, err)
}

func (w *LocalDisk) IsUploadOnly() bool {
	//We need to return false, otherwise sync point will never be saved
	return false
}
