package config

import (
	"reflect"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name                string
		conf                *Config
		expected            *Config
		expectedProvider    string
		expectedStorageOnly bool
		wantErr             bool
	}{
		{
			name: "local",
			conf: &Config{
				Provider: "local",
				Local: LocalConfig{
					SaveDir: "tmp",
				},
			},
			expected: &Config{
				Provider:    "local",
				SaveAsJson:  true,
				StorageOnly: true,
				ExportURL:   DefaultExportURL,
				Local: LocalConfig{
					SaveDir: "tmp",
				},
			},
		},
		{
			name: "local backwards compat",
			conf: &Config{
				Warehouse: "local",
				Local: LocalConfig{
					SaveDir: "tmp",
				},
			},
			expected: &Config{
				Provider:    "local",
				SaveAsJson:  true,
				StorageOnly: true,
				ExportURL:   DefaultExportURL,
				Local: LocalConfig{
					SaveDir: "tmp",
				},
			},
		},
		{
			name: "aws backwards compat",
			conf: &Config{
				Warehouse: "redshift",
				S3: S3Config{
					Bucket:  "bucket",
					Region:  "us-east-2",
					Timeout: duration{5 * time.Minute},
				},
			},
			expected: &Config{
				Provider:  "aws",
				ExportURL: DefaultExportURL,
				S3: S3Config{
					Bucket:  "bucket",
					Region:  "us-east-2",
					Timeout: duration{5 * time.Minute},
				},
				Redshift: RedshiftConfig{
					S3Region: "us-east-2",
				},
			},
		},
		{
			name: "aws backwards compat, storage only",
			conf: &Config{
				Warehouse: "redshift",
				S3: S3Config{
					Bucket:  "bucket",
					Region:  "us-east-2",
					Timeout: duration{5 * time.Minute},
					S3Only:  true,
				},
			},
			expected: &Config{
				Provider:    "aws",
				StorageOnly: true,
				ExportURL:   DefaultExportURL,
				S3: S3Config{
					Bucket:  "bucket",
					Region:  "us-east-2",
					Timeout: duration{5 * time.Minute},
				},
			},
		},
		{
			name: "gcp backwards compat",
			conf: &Config{
				Warehouse: "bigquery",
				GCS: GCSConfig{
					Bucket: "bucket",
				},
				BigQuery: BigQueryConfig{},
			},
			expected: &Config{
				Provider:  "gcp",
				ExportURL: DefaultExportURL,
				GCS: GCSConfig{
					Bucket: "bucket",
				},
				BigQuery: BigQueryConfig{},
			},
		},
		{
			name: "gcp backwards compat, storage only",
			conf: &Config{
				Warehouse: "bigquery",
				GCS: GCSConfig{
					Bucket:  "bucket",
					GCSOnly: true,
				},
			},
			expected: &Config{
				Provider:    "gcp",
				StorageOnly: true,
				ExportURL:   DefaultExportURL,
				GCS: GCSConfig{
					Bucket: "bucket",
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.conf); (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(tc.expected, tc.conf) {
				t.Errorf("config mismatch: want %v, got %v", tc.expected, tc.conf)
			}
		})
	}
}
