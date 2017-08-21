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
