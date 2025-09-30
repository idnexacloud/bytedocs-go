package parser

import "testing"

func TestGorillaMuxAnalyzerCapturesResponses(t *testing.T) {
	dir := "../../examples/gorilla-mux"

	metadata := getGorillaMuxHandlerMetadataByName("CreateUser", dir)
	if len(metadata.Responses) == 0 {
		t.Fatalf("expected responses for CreateUser, got none")
	}

	resp201, ok := metadata.Responses["201"]
	if !ok {
		t.Fatalf("expected 201 response for CreateUser")
	}
	if resp201.Schema == nil {
		t.Fatalf("expected schema for 201 response")
	}
	if resp201.Example == nil {
		t.Fatalf("expected example for 201 response")
	}

	metadataGet := getGorillaMuxHandlerMetadataByName("GetUsers", dir)
	if len(metadataGet.Responses) == 0 {
		t.Fatalf("expected responses for GetUsers")
	}
	resp200, ok := metadataGet.Responses["200"]
	if !ok {
		t.Fatalf("expected 200 response for GetUsers")
	}
	if resp200.Example == nil {
		t.Fatalf("expected example for 200 response: %+v", resp200)
	}
	exampleMap, ok := resp200.Example.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map example, got %T", resp200.Example)
	}
	usersRaw, ok := exampleMap["users"].([]interface{})
	if !ok || len(usersRaw) == 0 {
		t.Fatalf("expected users array in example, got %#v", resp200.Example)
	}
	firstUser, ok := usersRaw[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user object, got %#v", usersRaw[0])
	}
	if _, ok := firstUser["email"]; !ok {
		t.Fatalf("expected lowercase email key in example, got %#v", firstUser)
	}
}
