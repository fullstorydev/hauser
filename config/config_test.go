package config

import (
	"testing"
)

func makeRedshiftValidator(databaseSchema string) *RedshiftValidator {
	return &RedshiftValidator{
		RedshiftConfigFields{
			DatabaseSchema: databaseSchema,
		},
	}
}

func TestValidateDatabaseSchemaConfig(t *testing.T) {

	testCases := []struct {
		validator  *RedshiftValidator
		hasError   bool
		errMessage string
	}{
		{
			validator:  makeRedshiftValidator(""),
			hasError:   true,
			errMessage: "DatabaseSchema definition missing from Redshift configuration. More information: https://github.com/fullstorydev/hauser/blob/master/Redshift.md#database-schema-configuration",
		},
		{
			validator:  makeRedshiftValidator("test"),
			hasError:   false,
			errMessage: "",
		},
		{
			validator:  makeRedshiftValidator("search_path"),
			hasError:   false,
			errMessage: "",
		},
	}

	for _, tc := range testCases {
		err := tc.validator.ValidateDatabaseSchema()
		if tc.hasError && err == nil {
			t.Errorf("expected Redshift.ValidateDatabaseSchema() to return an error when config.Config.Redshift.DatabaseSchema is empty")
		}
		if tc.hasError && err.Error() != tc.errMessage {
			t.Errorf("expected Redshift.ValidateDatabaseSchema() to return \n%s \nwhen config.Config.Redshift.DatabaseSchema is empty, returned \n%s \ninstead", tc.errMessage, err)
		}
		if !tc.hasError && err != nil {
			t.Errorf("unexpected error thrown for Database %s: %s", tc.validator.DatabaseSchema, err)
		}
	}
}
