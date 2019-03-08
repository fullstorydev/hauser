package fullstory

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Session represents the FullStory session for a user.
type Session struct {
	UserID    int
	SessionID int
	Created   time.Time
	URL       string
}

func (s *Session) UnmarshalJSON(data []byte) error {
	aux := struct {
		UserID    int    `json:"UserId"`
		SessionID int    `json:"SessionId"`
		Created   int64  `json:"CreatedTime"`
		URL       string `json:"FsUrl"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	s.UserID = aux.UserID
	s.SessionID = aux.SessionID
	s.Created = time.Unix(aux.Created, 0)
	s.URL = aux.URL
	return nil
}

// Sessions returns a list of Session for the supplied parameters.
// If limit == -1, limit is ignored.
//
// For more details, see:
//   http://help.fullstory.com/develop-rest/137382-rest-api-retrieving-a-list-of-sessions-for-a-given-user-after-the-fact
func (c *Client) Sessions(limit int, uid, email string) ([]Session, error) {
	v := make(url.Values)
	if limit != -1 {
		v.Add("limit", strconv.Itoa(limit))
	}
	v.Add("uid", uid)
	v.Add("email", email)

	req, err := http.NewRequest("GET", c.BaseURL+"/sessions"+"?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doReq(req)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var s []Session
	if err := json.NewDecoder(body).Decode(&s); err != nil {
		return nil, err
	}
	return s, nil
}

// ExportMeta is metadata about ExportData.
type ExportMeta struct {
	Start time.Time
	Stop  time.Time
	ID    int
}

func (em *ExportMeta) UnmarshalJSON(data []byte) error {
	aux := struct {
		Start int64
		Stop  int64
		ID    int
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	em.Start = time.Unix(aux.Start, 0)
	em.Stop = time.Unix(aux.Stop, 0)
	em.ID = aux.ID
	return nil
}

// ExportList returns a list of metadata on completed data export bundles.
func (c *Client) ExportList(start time.Time) ([]ExportMeta, error) {
	v := make(url.Values)
	v.Add("start", fmt.Sprintf("%d", start.Unix()))

	req, err := http.NewRequest("GET", c.BaseURL+"/export/list"+"?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doReq(req)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var m map[string][]ExportMeta
	if err := json.NewDecoder(body).Decode(&m); err != nil {
		return nil, err
	}
	return m["exports"], nil
}

// ExportData represents data from the "/export/get" endpoint.
//
// For more details, see:
//   http://help.fullstory.com/develop-rest/data-export-api
type ExportData io.ReadCloser

// ExportData returns the data export bundle specified by id.
//
// If the client's HTTP Transport has DisableCompression set to true, the
// caller should treat the returned ExportData as gzipped JSON. Otherwise,
// ExportData is JSON that has already been uncompressed.
//
// The caller is responsible for closing the returned ExportData if the returned
// error is nil.
func (c *Client) ExportData(id int, modifyReq ...func(r *http.Request)) (ExportData, error) {
	v := make(url.Values)
	v.Add("id", strconv.Itoa(id))

	req, err := http.NewRequest("GET", c.BaseURL+"/export/get"+"?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}

	for _, mr := range modifyReq {
		mr(req)
	}

	return c.doReq(req)
}
