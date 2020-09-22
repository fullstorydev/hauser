package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

var ErrExportNotReady = errors.New("export not ready to download")

type ExportError struct {
	Details string
}

func (e ExportError) Error() string {
	return fmt.Sprintf("failed to complete export: %s", e.Details)
}

type timeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type createParams struct {
	SegmentId        string    `json:"segmentId"`
	Type             string    `json:"type"`
	Format           string    `json:"format"`
	SegmentTimeRange timeRange `json:"segmentTimeRange"`
	TimeRange        timeRange `json:"timeRange"`
}

type createSegmentResponse struct {
	Id string `json:"operationId"`
}

type getExportResultsResponse struct {
	Location string `json:"location"`
}

func (c *Client) CreateExport(start time.Time, end time.Time) (string, error) {
	params := createParams{
		SegmentId: "everyone",
		Type:      "TYPE_EVENT",
		Format:    "FORMAT_JSON",
		// Specify a segment time range with empty values to indicate "All Time"
		SegmentTimeRange: timeRange{Start: "", End: ""},
		// Limit the exported data to the requested time range.
		TimeRange: timeRange{
			Start: start.UTC().Format(time.RFC3339),
			End:   end.UTC().Format(time.RFC3339),
		},
	}
	reqBody, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/segments/v1/exports", c.Config.ApiURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	resBody, err := c.doReq(req)
	if err != nil {
		// Status 429, 499, 500 -- retry
		// Status else fail
		return "", err
	}
	defer resBody.Close()
	resp := createSegmentResponse{}
	err = json.NewDecoder(resBody).Decode(&resp)
	return resp.Id, err
}

func (c *Client) GetExportProgress(operationId string) (int, bool, error) {
	resp, err := c.getExportOperation(operationId)
	if err != nil {
		return 0, false, err
	}
	if resp.Type != operationSearchExport {
		return 0, false, errors.New("bad operation id")
	}
	if resp.State == operationFailed {
		return 0, false, ExportError{Details: resp.ErrorDetails}
	}
	return resp.EstimatedPctComplete, resp.State == operationComplete, nil
}

func (c *Client) GetExport(operationId string) (io.ReadCloser, error) {
	resp, err := c.getExportOperation(operationId)
	if err != nil {
		return nil, err
	}
	if resp.State != operationComplete {
		return nil, ErrExportNotReady
	}
	return c.getExportStream(resp.Results.SearchExportId)
}

func (c *Client) getExportStream(exportId string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/search/v1/exports/%s/results", c.Config.ApiURL, exportId)
	req, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()
	rsp := &getExportResultsResponse{}
	if err := json.NewDecoder(req.Body).Decode(&rsp); err != nil {
		return nil, err
	}

	streamRsp, err := http.Get(rsp.Location)
	if err != nil {
		return nil, err
	}
	if streamRsp.StatusCode != http.StatusOK {
		return nil, StatusError{
			Status:     streamRsp.Status,
			StatusCode: streamRsp.StatusCode,
			RetryAfter: time.Duration(getRetryAfter(streamRsp)) * time.Second,
			Body:       nil,
		}
	}
	return streamRsp.Body, nil
}
