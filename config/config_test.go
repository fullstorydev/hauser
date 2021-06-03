package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/fullstorydev/hauser/testing/testutils"
)

func TestValidate(t *testing.T) {

	now := time.Date(2020, 10, 7, 0, 0, 0, 0, time.UTC)
	getNow := func() time.Time {
		return now
	}

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
				Provider:       "local",
				StorageOnly:    true,
				ApiURL:         DefaultApiURL,
				ExportDuration: Duration{time.Hour},
				ExportDelay:    Duration{24 * time.Hour},
				StartTime:      now.Add(-1 * 24 * 30 * time.Hour),
				Local: LocalConfig{
					SaveDir: "tmp",
				},
			},
		},
		{
			name: "local backwards compat",
			conf: &Config{
				Warehouse:  "local",
				SaveAsJson: true,
				Local: LocalConfig{
					SaveDir: "tmp",
				},
			},
			expected: &Config{
				Provider:       "local",
				SaveAsJson:     true,
				StorageOnly:    true,
				ApiURL:         DefaultApiURL,
				ExportDuration: Duration{time.Hour},
				ExportDelay:    Duration{24 * time.Hour},
				StartTime:      now.Add(-1 * 24 * 30 * time.Hour),
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
					Timeout: Duration{5 * time.Minute},
				},
			},
			expected: &Config{
				Provider:       "aws",
				ApiURL:         DefaultApiURL,
				ExportDuration: Duration{time.Hour},
				ExportDelay:    Duration{24 * time.Hour},
				StartTime:      now.Add(-1 * 24 * 30 * time.Hour),
				S3: S3Config{
					Bucket:  "bucket",
					Region:  "us-east-2",
					Timeout: Duration{5 * time.Minute},
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
					Timeout: Duration{5 * time.Minute},
					S3Only:  true,
				},
			},
			expected: &Config{
				Provider:       "aws",
				StorageOnly:    true,
				ApiURL:         DefaultApiURL,
				ExportDuration: Duration{time.Hour},
				ExportDelay:    Duration{24 * time.Hour},
				StartTime:      now.Add(-1 * 24 * 30 * time.Hour),
				S3: S3Config{
					Bucket:  "bucket",
					Region:  "us-east-2",
					Timeout: Duration{5 * time.Minute},
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
				Provider:       "gcp",
				ApiURL:         DefaultApiURL,
				ExportDuration: Duration{time.Hour},
				ExportDelay:    Duration{24 * time.Hour},
				StartTime:      now.Add(-1 * 24 * 30 * time.Hour),
				GCS: GCSConfig{
					Bucket: "bucket",
				},
				BigQuery: BigQueryConfig{},
			},
		},
		{
			name: "gcp backwards compat, storage only",
			conf: &Config{
				Warehouse:  "bigquery",
				SaveAsJson: true,
				GCS: GCSConfig{
					Bucket:  "bucket",
					GCSOnly: true,
				},
			},
			expected: &Config{
				Provider:       "gcp",
				StorageOnly:    true,
				SaveAsJson:     true,
				ApiURL:         DefaultApiURL,
				ExportDuration: Duration{time.Hour},
				ExportDelay:    Duration{24 * time.Hour},
				StartTime:      now.Add(-1 * 24 * 30 * time.Hour),
				GCS: GCSConfig{
					Bucket: "bucket",
				},
			},
		},
		{
			name: "bad delay Duration",
			conf: &Config{
				Provider: "gcp",
				GCS: GCSConfig{
					Bucket: "bucket",
				},
				BigQuery:    BigQueryConfig{},
				ExportDelay: Duration{time.Minute},
			},
			wantErr: true,
		},
		{
			name: "bad export Duration",
			conf: &Config{
				Provider: "gcp",
				GCS: GCSConfig{
					Bucket: "bucket",
				},
				BigQuery:       BigQueryConfig{},
				ExportDuration: Duration{7 * time.Minute},
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.conf, getNow)
			if tc.wantErr {
				testutils.Assert(t, err != nil, "expected error")
			} else {
				testutils.Assert(t, err == nil, "unexpected error: %s", err)
				if !reflect.DeepEqual(tc.expected, tc.conf) {
					t.Errorf("config mismatch: want %v, got %v", tc.expected, tc.conf)
				}
			}
		})
	}
}
