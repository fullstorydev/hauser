package warehouse

import (
	"reflect"
	"testing"
	"time"

	"github.com/fullstorydev/hauser/testing/testutils"
)

var legacyColumns = []string{
	"eventcustomname",
	"eventstart",
	"eventtype",
	"eventtargettext",
	"eventtargetselectortok",
	"eventmodfrustrated",
	"eventmoddead",
	"eventmoderror",
	"eventmodsuspicious",
	"indvid",
	"pageclusterid",
	"pageurl",
	"pageduration",
	"pageactiveduration",
	"pagerefererurl",
	"pagelatlong",
	"pageagent",
	"pageip",
	"pagebrowser",
	"pagedevice",
	"pageoperatingsystem",
	"pagenuminfos",
	"pagenumwarnings",
	"pagenumerrors",
	"sessionid",
	"pageid",
	"userappkey",
	"useremail",
	"userdisplayname",
	"userid",
	"customvars",
	"loaddomcontenttime",
	"loadfirstpainttime",
	"loadeventtime",
}

var (
	stringType = reflect.TypeOf("")
	int64Type  = reflect.TypeOf(int64(0))
	timeType   = reflect.TypeOf(time.Time{})
)

func TestSchema_ReconcileWithExisting(t *testing.T) {
	testCases := []struct {
		name   string
		cols   []string
		expect Schema
	}{
		{
			name: "Legacy with new columns",
			cols: legacyColumns,
			expect: []WarehouseField{
				{"EventCustomName", "EventCustomName", stringType, ""},
				{"EventStart", "EventStart", timeType, ""},
				{"EventType", "EventType", stringType, ""},
				{"EventTargetText", "EventTargetText", stringType, ""},
				{"EventTargetSelectorTok", "EventTargetSelectorTok", stringType, ""},
				{"EventModFrustrated", "EventModFrustrated", int64Type, ""},
				{"EventModDead", "EventModDead", int64Type, ""},
				{"EventModError", "EventModError", int64Type, ""},
				{"EventModSuspicious", "EventModSuspicious", int64Type, ""},
				{"IndvId", "IndvId", int64Type, ""},
				{"PageClusterId", "PageClusterId", int64Type, ""},
				{"PageUrl", "PageUrl", stringType, ""},
				{"PageDuration", "PageDuration", int64Type, ""},
				{"PageActiveDuration", "PageActiveDuration", int64Type, ""},
				{"PageRefererUrl", "PageRefererUrl", stringType, ""},
				{"PageLatLong", "PageLatLong", stringType, ""},
				{"PageAgent", "PageUserAgent", stringType, ""},
				{"PageIp", "PageIp", stringType, ""},
				{"PageBrowser", "PageBrowser", stringType, ""},
				{"PageDevice", "PageDevice", stringType, ""},
				{"PageOperatingSystem", "PageOperatingSystem", stringType, ""},
				{"PageNumInfos", "PageNumInfos", int64Type, ""},
				{"PageNumWarnings", "PageNumWarnings", int64Type, ""},
				{"PageNumErrors", "PageNumErrors", int64Type, ""},
				{"SessionId", "SessionId", int64Type, ""},
				{"PageId", "PageId", int64Type, ""},
				{"UserAppKey", "UserAppKey", stringType, ""},
				{"UserEmail", "UserEmail", stringType, ""},
				{"UserDisplayName", "UserDisplayName", stringType, ""},
				{"UserId", "UserId", int64Type, ""},
				{"CustomVars", "CustomVars", stringType, ""},
				{"LoadDomContentTime", "LoadDomContentTime", int64Type, ""},
				{"LoadFirstPaintTime", "LoadFirstPaintTime", int64Type, ""},
				{"LoadEventTime", "LoadEventTime", int64Type, ""},
				{"UserCreated", "UserCreated", timeType, ""},
				{"EventTargetSelector", "EventTargetSelector", stringType, ""},
				{"SessionStart", "SessionStart", timeType, ""},
				{"PageStart", "PageStart", timeType, ""},
				{"PagePlatform", "PagePlatform", stringType, ""},
				{"PageScreenWidth", "PageScreenWidth", int64Type, ""},
				{"PageScreenHeight", "PageScreenHeight", int64Type, ""},
				{"PageViewportWidth", "PageViewportWidth", int64Type, ""},
				{"PageViewportHeight", "PageViewportHeight", int64Type, ""},
			},
		},
		{
			name: "brand new schema",
			cols: []string{},
			expect: []WarehouseField{
				{"IndvId", "IndvId", int64Type, ""},
				{"UserId", "UserId", int64Type, ""},
				{"SessionId", "SessionId", int64Type, ""},
				{"PageId", "PageId", int64Type, ""},
				{"UserCreated", "UserCreated", timeType, ""},
				{"UserAppKey", "UserAppKey", stringType, ""},
				{"UserDisplayName", "UserDisplayName", stringType, ""},
				{"UserEmail", "UserEmail", stringType, ""},
				{"EventStart", "EventStart", timeType, ""},
				{"EventType", "EventType", stringType, ""},
				{"EventCustomName", "EventCustomName", stringType, ""},
				{"EventTargetText", "EventTargetText", stringType, ""},
				{"EventTargetSelector", "EventTargetSelector", stringType, ""},
				{"EventModFrustrated", "EventModFrustrated", int64Type, ""},
				{"EventModDead", "EventModDead", int64Type, ""},
				{"EventModError", "EventModError", int64Type, ""},
				{"EventModSuspicious", "EventModSuspicious", int64Type, ""},
				{"SessionStart", "SessionStart", timeType, ""},
				{"PageStart", "PageStart", timeType, ""},
				{"PageDuration", "PageDuration", int64Type, ""},
				{"PageActiveDuration", "PageActiveDuration", int64Type, ""},
				{"PageUrl", "PageUrl", stringType, ""},
				{"PageRefererUrl", "PageRefererUrl", stringType, ""},
				{"PageIp", "PageIp", stringType, ""},
				{"PageLatLong", "PageLatLong", stringType, ""},
				{"PageUserAgent", "PageUserAgent", stringType, ""},
				{"PageBrowser", "PageBrowser", stringType, ""},
				{"PageDevice", "PageDevice", stringType, ""},
				{"PagePlatform", "PagePlatform", stringType, ""},
				{"PageOperatingSystem", "PageOperatingSystem", stringType, ""},
				{"PageScreenWidth", "PageScreenWidth", int64Type, ""},
				{"PageScreenHeight", "PageScreenHeight", int64Type, ""},
				{"PageViewportWidth", "PageViewportWidth", int64Type, ""},
				{"PageViewportHeight", "PageViewportHeight", int64Type, ""},
				{"PageNumInfos", "PageNumInfos", int64Type, ""},
				{"PageNumWarnings", "PageNumWarnings", int64Type, ""},
				{"PageNumErrors", "PageNumErrors", int64Type, ""},
				{"PageClusterId", "PageClusterId", int64Type, ""},
				{"LoadDomContentTime", "LoadDomContentTime", int64Type, ""},
				{"LoadEventTime", "LoadEventTime", int64Type, ""},
				{"LoadFirstPaintTime", "LoadFirstPaintTime", int64Type, ""},
				{"CustomVars", "CustomVars", stringType, ""},
			},
		},
		{
			name: "someone added some columns",
			cols: []string{"preexisting", "columns", "userid"},
			expect: []WarehouseField{
				{"preexisting", "", nil, ""},
				{"columns", "", nil, ""},
				{"UserId", "UserId", int64Type, ""},
				{"IndvId", "IndvId", int64Type, ""},
				{"SessionId", "SessionId", int64Type, ""},
				{"PageId", "PageId", int64Type, ""},
				{"UserCreated", "UserCreated", timeType, ""},
				{"UserAppKey", "UserAppKey", stringType, ""},
				{"UserDisplayName", "UserDisplayName", stringType, ""},
				{"UserEmail", "UserEmail", stringType, ""},
				{"EventStart", "EventStart", timeType, ""},
				{"EventType", "EventType", stringType, ""},
				{"EventCustomName", "EventCustomName", stringType, ""},
				{"EventTargetText", "EventTargetText", stringType, ""},
				{"EventTargetSelector", "EventTargetSelector", stringType, ""},
				{"EventModFrustrated", "EventModFrustrated", int64Type, ""},
				{"EventModDead", "EventModDead", int64Type, ""},
				{"EventModError", "EventModError", int64Type, ""},
				{"EventModSuspicious", "EventModSuspicious", int64Type, ""},
				{"SessionStart", "SessionStart", timeType, ""},
				{"PageStart", "PageStart", timeType, ""},
				{"PageDuration", "PageDuration", int64Type, ""},
				{"PageActiveDuration", "PageActiveDuration", int64Type, ""},
				{"PageUrl", "PageUrl", stringType, ""},
				{"PageRefererUrl", "PageRefererUrl", stringType, ""},
				{"PageIp", "PageIp", stringType, ""},
				{"PageLatLong", "PageLatLong", stringType, ""},
				{"PageUserAgent", "PageUserAgent", stringType, ""},
				{"PageBrowser", "PageBrowser", stringType, ""},
				{"PageDevice", "PageDevice", stringType, ""},
				{"PagePlatform", "PagePlatform", stringType, ""},
				{"PageOperatingSystem", "PageOperatingSystem", stringType, ""},
				{"PageScreenWidth", "PageScreenWidth", int64Type, ""},
				{"PageScreenHeight", "PageScreenHeight", int64Type, ""},
				{"PageViewportWidth", "PageViewportWidth", int64Type, ""},
				{"PageViewportHeight", "PageViewportHeight", int64Type, ""},
				{"PageNumInfos", "PageNumInfos", int64Type, ""},
				{"PageNumWarnings", "PageNumWarnings", int64Type, ""},
				{"PageNumErrors", "PageNumErrors", int64Type, ""},
				{"PageClusterId", "PageClusterId", int64Type, ""},
				{"LoadDomContentTime", "LoadDomContentTime", int64Type, ""},
				{"LoadEventTime", "LoadEventTime", int64Type, ""},
				{"LoadFirstPaintTime", "LoadFirstPaintTime", int64Type, ""},
				{"CustomVars", "CustomVars", stringType, ""},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			baseSchema := MakeSchema(BaseExportFields{})
			updatedSchema := baseSchema.ReconcileWithExisting(tc.cols)
			testutils.Assert(t, tc.expect.Equals(updatedSchema), "wrong schema:\nwant %#v,\ngot  %#v", tc.expect, updatedSchema)
		})
	}
}
