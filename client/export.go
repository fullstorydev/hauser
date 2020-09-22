package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

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

	req, err := http.NewRequest("GET", c.Config.ExportURL+"/export/list"+"?"+v.Encode(), nil)
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

	req, err := http.NewRequest("GET", c.Config.ExportURL+"/export/get"+"?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}

	for _, mr := range modifyReq {
		mr(req)
	}

	return c.doReq(req)
}
