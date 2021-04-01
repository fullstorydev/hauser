package warehouse

import "testing"

func TestGetBucketAndKey(t *testing.T) {
	testCases := []struct {
		s3Config  string
		fileName  string
		expBucket string
		expKey    string
	}{
		{
			s3Config:  "plainbucket",
			fileName:  "data.csv",
			expBucket: "plainbucket",
			expKey:    "data.csv",
		},
		{
			s3Config:  "hasslash/",
			fileName:  "data.csv",
			expBucket: "hasslash",
			expKey:    "data.csv",
		},
		{
			s3Config:  "hasslash/withpath",
			fileName:  "data.csv",
			expBucket: "hasslash",
			expKey:    "withpath/data.csv",
		}, {
			s3Config:  "hasslash/withpathwithslash/",
			fileName:  "data.csv",
			expBucket: "hasslash",
			expKey:    "withpathwithslash/data.csv",
		},
	}
	for _, tc := range testCases {
		bucketName, key := getBucketAndKey(tc.s3Config, tc.fileName)
		if bucketName != tc.expBucket {
			t.Errorf("getBucketAndKey(%s, %s) returned %s for bucketName, expected %s", tc.s3Config, tc.fileName, bucketName, tc.expBucket)
		}
		if key != tc.expKey {
			t.Errorf("getBucketAndKey(%s, %s) returned %s for key, expected %s", tc.s3Config, tc.fileName, key, tc.expKey)
		}
	}
}
