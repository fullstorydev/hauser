package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ExportError struct {
	Details string
}

func (e ExportError) Error() string {
	return fmt.Sprintf("failed to complete export: %s", e.Details)
}

type exportType string

const exportType_Event exportType = "TYPE_EVENT"

type exportFormat string

const exportFormat_Json exportFormat = "FORMAT_JSON"

type timeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type createParams struct {
	SegmentId        string       `json:"segmentId"`
	Type             exportType   `json:"type"`
	Format           exportFormat `json:"format"`
	SegmentTimeRange timeRange    `json:"segmentTimeRange"`
	TimeRange        timeRange    `json:"timeRange"`
	Fields           []string     `json:"fields"`
}

type createSegmentResponse struct {
	OperationId string `json:"operationId"`
}

type getExportResultsResponse struct {
	Location string `json:"location"`
}

func (c *Client) CreateExport(start time.Time, end time.Time, fields []string) (string, error) {
	params := createParams{
		SegmentId: "everyone",
		Type:      exportType_Event,
		Format:    exportFormat_Json,
		// Specify a segment time range with empty values to indicate "All Time"
		SegmentTimeRange: timeRange{Start: "", End: ""},
		// Limit the exported data to the requested time range.
		TimeRange: timeRange{
			Start: start.UTC().Format(time.RFC3339),
			End:   end.UTC().Format(time.RFC3339),
		},
		Fields: fields,
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

	if c.createRequestModifier != nil {
		c.createRequestModifier(req)
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
	return resp.OperationId, err
}

func (c *Client) GetExportProgress(operationId string) (int, string, error) {
	resp, err := c.getExportOperation(operationId)
	if err != nil {
		return 0, "", err
	}
	if resp.State == operationComplete {
		return resp.EstimatedPctComplete, resp.Results.SearchExportId, nil
	}
	return resp.EstimatedPctComplete, "", nil
}

func (c *Client) GetExport(exportId string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/search/v1/exports/%s/results", c.Config.ApiURL, exportId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	body, err := c.doReq(req)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	rsp := &getExportResultsResponse{}
	if err := json.NewDecoder(body).Decode(&rsp); err != nil {
		return nil, err
	}

	// Use a vanilla http client for downloading the stream since auth
	// is built into the URL itself.
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
