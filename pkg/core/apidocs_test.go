package core

import "testing"

func TestConvertPathToOpenAPI_GorillaMuxRegex(t *testing.T) {
	in := "/api/v1/users/{id:[0-9]+}"
	expected := "/api/v1/users/{id}"
	if got := convertPathToOpenAPI(in); got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestConvertPathToOpenAPI_MixedFormats(t *testing.T) {
	in := "api/<tenant>/:resource/{slug:[a-z-]+}"
	expected := "/api/{tenant}/{resource}/{slug}"
	if got := convertPathToOpenAPI(in); got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}
