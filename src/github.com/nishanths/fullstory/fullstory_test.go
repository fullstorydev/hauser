package fullstory

import (
	"compress/gzip"
	"io/ioutil"
	"testing"
	"time"
)

func TestSessions(t *testing.T) {
	t.Parallel()
	s, err := client.Sessions(-1, "-", "john@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 1 {
		t.Fatalf("len(sessions). got %d, want %d", len(s), 1)
	}
	if s[0].UserID != 1234567890 {
		t.Fatalf("UserID. got %d, want %d", s[0].UserID, 1234567890)
	}
	if s[0].SessionID != 1234567890 {
		t.Fatalf("SessionID. got %d, want %d", s[0].SessionID, 1234567890)
	}
	if s[0].Created.Unix() != 1411492739 {
		t.Fatalf("Created timestamp. got %d, want %d", s[0].Created.Unix(), 1411492739)
	}
	if s[0].URL != "https://www.fullstory.com/ui/ORG_ID/discover/session/1234567890:1234567890" {
		t.Fatalf("URL. got %q, want %q", s[0].URL, "https://www.fullstory.com/ui/ORG_ID/discover/session/1234567890:1234567890")
	}
}

func TestExportList(t *testing.T) {
	t.Parallel()
	el, err := client.ExportList(time.Unix(144798399, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(el) != 3 {
		t.Fatalf("len(el). got %d, want %d", len(el), 3)
	}
	if el[2].Start.Unix() != 1448157600 {
		t.Fatalf("el[2].Start timestamp. got %d, want %d", el[2].Start.Unix(), 1448157600)
	}
	if el[2].Stop.Unix() != 1448244000 {
		t.Fatalf("el[2].Stop timestamp. got %d, want %d", el[2].Stop.Unix(), 1448244000)
	}
	if el[2].ID != 456789123 {
		t.Fatalf("el[2]ID. got %d, want %d", el[2].ID, 456789123)
	}
}

func TestExportData(t *testing.T) {
	t.Parallel()
	data, err := client.ExportData(12345)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()

	// Automatically decompresses.

	b, err := ioutil.ReadAll(data)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)

	expect := `{"foo":bar, "hello:world", "answer":42, "question":null}`

	if s != expect {
		t.Fatalf("got %q, want %q", s, expect)
	}
}

func TestExportDataGzip(t *testing.T) {
	t.Parallel()
	data, err := client2.ExportData(12345)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()
	r, err := gzip.NewReader(data)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	// Requires manual decompression.

	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)

	expect := `{"foo":bar, "hello:world", "answer":42, "question":null}`
	if s != expect {
		t.Fatalf("got %q, want %q", s, expect)
	}
}
