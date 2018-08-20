package fullstory

import (
	"net/http"
	"testing"
)

func TestClient_doReq(t *testing.T) {
	req, err := http.NewRequest("GET", srv.URL+"/non-existent", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.doReq(req)
	se, ok := err.(StatusError)
	if !ok {
		t.Fatalf("err. got %T, want StatusError", err)
	}
	if se.StatusCode != http.StatusNotFound {
		t.Fatalf("se.StatusCode. got %d, want %d", se.StatusCode, http.StatusNotFound)
	}
}

func TestClient_getRetryAfter(t *testing.T) {
	testCases := []struct {
		retryHeader string
		expResult   int
	}{
		{
			retryHeader: "",
			expResult:   0,
		},
		{
			retryHeader: "unparseable",
			expResult:   0,
		},
		{
			retryHeader: "15",
			expResult:   15,
		},
		{
			retryHeader: "60",
			expResult:   60,
		},
	}

	for i, tc := range testCases {
		resp := http.Response{Header: http.Header{"Retry-After": []string{tc.retryHeader}}}
		result := getRetryAfter(&resp)
		if result != tc.expResult {
			t.Errorf("Expected %d, got %d on test case %d", tc.expResult, result, i)
		}
	}
}
