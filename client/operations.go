package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type operationType string

const operationSearchExport operationType = "SEARCH_EXPORT"

type operationState string

const operationFailed operationState = "FAILED"
const operationComplete operationState = "COMPLETED"

type exportOperationResults struct {
	Expires        string `json:"expires"`
	SearchExportId string `json:"searchExportId"`
}

type operationsResponse struct {
	Type    operationType          `json:"type"`
	Results exportOperationResults `json:"results"`
	State   operationState         `json:"state"`
	// If state is failed, this will contain the reason why the export failed.
	ErrorDetails         string `json:"errorDetails"`
	EstimatedPctComplete int    `json:"estimatePctComplete"`
}

func (o *operationsResponse) Err() error {
	if o == nil || o.State != operationFailed {
		return nil
	}
	return ExportError{Details: o.ErrorDetails}
}

func (c *Client) getExportOperation(operationId string) (*operationsResponse, error) {
	resp := &operationsResponse{}
	url := fmt.Sprintf("%s/operations/v1/%s", c.Config.ApiURL, operationId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resBody, err := c.doReq(req)
	if err != nil {
		return nil, err
	}
	defer resBody.Close()
	if err := json.NewDecoder(resBody).Decode(resp); err != nil {
		return nil, err
	}
	if resp.Type != operationSearchExport {
		return nil, errors.New("operation id does not correspond to an export")
	}
	return resp, resp.Err()
}
