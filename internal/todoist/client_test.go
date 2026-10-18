package todoist

import "testing"

func TestNewAPIError_ParsesErrorCodeAndTag(t *testing.T) {
	body := []byte(`{"error":"Project not found","error_code":478,"error_tag":"NOT_FOUND","http_code":404}`)
	e := newAPIError(404, body)

	if e.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", e.StatusCode)
	}
	if e.ErrorCode != 478 {
		t.Errorf("ErrorCode = %d, want 478", e.ErrorCode)
	}
	if e.ErrorTag != "NOT_FOUND" {
		t.Errorf("ErrorTag = %q, want NOT_FOUND", e.ErrorTag)
	}
	if e.Body != string(body) {
		t.Errorf("Body not preserved: %q", e.Body)
	}
}

func TestNewAPIError_NonJSONBodyIsZeroValued(t *testing.T) {
	e := newAPIError(500, []byte("Internal Server Error"))

	if e.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", e.StatusCode)
	}
	if e.ErrorCode != 0 || e.ErrorTag != "" {
		t.Errorf("expected zero-valued ErrorCode/ErrorTag for non-JSON body, got %d/%q", e.ErrorCode, e.ErrorTag)
	}
	if e.Body != "Internal Server Error" {
		t.Errorf("Body not preserved: %q", e.Body)
	}
}

func TestAPIError_PremiumOnlyIsReadable(t *testing.T) {
	body := []byte(`{"error":"Deadlines are a premium only feature","error_code":32,"error_extra":{"event_id":"x","retry_after":4},"error_tag":"PREMIUM_ONLY","http_code":403}`)
	got := newAPIError(403, body).Error()
	want := "todoist: Deadlines are a premium only feature — this needs a paid Todoist plan"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_OtherErrorsKeepRawBody(t *testing.T) {
	body := []byte(`{"error":"Project not found","error_code":478,"error_tag":"NOT_FOUND","http_code":404}`)
	if got := newAPIError(404, body).Error(); got != "todoist: HTTP 404: "+string(body) {
		t.Errorf("Error() = %q", got)
	}
}
