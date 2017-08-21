package warehouse

import "fmt"

type Field struct {
	Name   string
	DBType string
}

func (f Field) String() string {
	return fmt.Sprintf("%s %s", f.Name, f.DBType)
}

var (
	ExportSchema = []Field{
		Field{"EventStart", "TIMESTAMP"},
		Field{"EventType", "varchar(max)"},
		Field{"EventTargetText", "varchar(max)"},
		Field{"EventTargetSelectorTok", "varchar(max)"},
		Field{"EventModFrustrated", "BIGINT"},
		Field{"EventModDead", "BIGINT"},
		Field{"EventModError", "BIGINT"},
		Field{"EventModSuspicious", "BIGINT"},
		Field{"IndvId", "BIGINT"},
		Field{"PageUrl", "varchar(max)"},
		Field{"PageDuration", "BIGINT"},
		Field{"PageActiveDuration", "BIGINT"},
		Field{"PageRefererUrl", "varchar(max)"},
		Field{"PageLatLong", "varchar(max)"},
		Field{"PageAgent", "varchar(max)"},
		Field{"PageIp", "varchar(max)"},
		Field{"PageBrowser", "varchar(max)"},
		Field{"PageDevice", "varchar(max)"},
		Field{"PageOperatingSystem", "varchar(max)"},
		Field{"PageNumInfos", "BIGINT"},
		Field{"PageNumWarnings", "BIGINT"},
		Field{"PageNumErrors", "BIGINT"},
		Field{"SessionId", "BIGINT"},
		Field{"PageId", "BIGINT"},
		Field{"UserAppKey", "varchar(max)"},
		Field{"UserEmail", "varchar(max)"},
		Field{"UserDisplayName", "varchar(max)"},
		Field{"UserId", "BIGINT"},
	}

	CustomVars = Field{"CustomVars", "varchar(max)"}

	SyncTableSchema = []Field{
		Field{"ID", "BIGINT"},
		Field{"Processed", "TIMESTAMP"},
		Field{"BundleEndTime", "TIMESTAMP"},
	}
)
