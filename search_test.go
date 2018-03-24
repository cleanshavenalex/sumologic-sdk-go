package sumologic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

//https://help.sumologic.com/APIs/Search-Job-API/About-the-Search-Job-API#Creating_a_search_job
func TestStartSearch(t *testing.T) {
	testStartSearch := StartSearchRequest{
		Query:    "_sourceCategory=test/sumo",
		From:     fmt.Sprintf(time.Now().UTC().Format(time.RFC3339)),
		To:       fmt.Sprintf(time.Now().UTC().Format(time.RFC3339)),
		TimeZone: "PST",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		if r.Method != "POST" {
			t.Errorf("Expected ‘POST’ request, got ‘%s’", r.Method)
		}
		expectedURL := fmt.Sprintf("/search/jobs")
		if r.URL.EscapedPath() != expectedURL {
			t.Errorf("Expected request to ‘%s’, got ‘%s’", expectedURL, r.URL.EscapedPath())
		}
		body, _ := json.Marshal(SearchJob{
			Status:  202,
			ID:      "TestStartSearchJob",
			Code:    "searchjob.valid",
			Message: "Search Running",
		})
		w.Write(body)
	}))
	defer ts.Close()

	c, err := NewClient("accessToken", ts.URL)
	if err != nil {
		t.Errorf("NewClient() returned an error: %s", err)
		return
	}

	startSearchResponse, err := c.StartSearch(testStartSearch)
	if err != nil {
		t.Errorf("StartSearch() returned an error: %s", err)
		return
	}

	if startSearchResponse.Message != "Search Running" {
		t.Errorf("StartSearch() expected message 'Search Running', got `%v`", startSearchResponse.Message)
		return
	}
}

func TestGetSearchStatus(t *testing.T) {

	testSearchJob := SearchJob{
		ID:      "testsearchjob",
		cookies: nil,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method != "GET" {
			t.Errorf("Expected ‘GET’ request, got ‘%s’", r.Method)
		}
		expectedURL := fmt.Sprintf("/search/jobs/%s", testSearchJob.ID)
		if r.URL.EscapedPath() != expectedURL {
			t.Errorf("Expected request to ‘%s’, got ‘%s’", expectedURL, r.URL.EscapedPath())
		}
		body, _ := json.Marshal(SearchJobStatusResponse{
			State: GatheringResults,
		})
		w.Write(body)
	}))
	defer ts.Close()

	c, err := NewClient("accessToken", ts.URL)
	if err != nil {
		t.Errorf("NewClient() returned an error: %s", err)
		return
	}
	testSearchJob.apiClient = c
	_, err = testSearchJob.GetStatus()
	if err != nil {
		t.Errorf("testSearchJob.GetStatus() returned an error: %s", err)
		return
	}

}

func TestGetSearchResults(t *testing.T) {

	testSearchJob := SearchJob{
		ID: "testGetSearchResultsFakeId",
	}
	// mock Search Result response
	// intended to create a slimmed down verion of the sample from the sumo search api docs
	// https://help.sumologic.com/APIs/Search-Job-API/About-the-Search-Job-API
	// {
	// "fields":[
	//   {
	//      "name":"_messageid",
	//      "fieldType":"long",
	//      "keyField":false
	//   }, ...
	//   {
	//      "name":"_raw",
	//      "fieldType":"string",
	//      "keyField":false
	//   }
	// ],
	//  "messages":[
	// 	{
	// 	   "map":{
	// 		  "_messageid":"-9223372036854773763",
	// 		  "_raw":"2013-01-28 13:09:10,333 -0800 INFO  [module=SERVICE] [logger=util.scala.zk.discovery.AWSServiceRegistry] [thread=pool-1-thread-1] FINISHED findRunningInstances(ListBuffer((Service: name: elasticache-1, defaultProps: Map()), (Service: name: userAndOrgCache, defaultProps: Map()), (Service: name: rds_cloudcollector, defaultProps: Map()))) returning Map((Service: name: elasticache-1, defaultProps: Map()) -> [], (Service: name: userAndOrgCache, defaultProps: Map()) -> [], (Service: name: rds_cloudcollector, defaultProps: Map()) -> []) after 1515 ms",
	// 	   }
	// 	},
	// 	{
	// 	   "map":{
	// 		  "_messageid":"-9223372036854773772",
	// 		  "_raw":"2013-01-28 13:04:09,529 -0800 INFO  [module=SERVICE] [logger=com.netflix.config.sources.DynamoDbConfigurationSource] [thread=pollingConfigurationSource] Successfully polled Dynamo for a new configuration based on table:raychaser-chiapetProperties",
	// 	   }
	// 	}
	//  ]
	// }
	var fields []SearchJobResultField
	var messages []SearchJobResultMessage
	field0 := SearchJobResultField{
		Name:      "_messageid",
		FieldType: "long",
		KeyField:  false,
	}
	fields = append(fields, field0)
	field1 := SearchJobResultField{
		Name:      "_raw",
		FieldType: "string",
	}
	fields = append(fields, field1)
	message0map := make(map[string]interface{})
	message1map := make(map[string]interface{})
	message0map["_messageid"] = "messsageZero"
	message0map["_raw"] = `{ "host" : "test.host0", "client_ip" : "0.0.0.0", "number" : "0" }`
	message0 := SearchJobResultMessage{
		Map: message0map,
	}
	messages = append(messages, message0)
	message1map["_messageid"] = "messageOne"
	message1map["_raw"] = `{ "host" : "test.host1", "client_ip" : "127.0.0.1", "number" : "1" }`
	message1 := SearchJobResultMessage{
		Map: message1map,
	}
	messages = append(messages, message1)
	testSearchJobResult := SearchJobResult{
		Fields:   fields,
		Messages: messages,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method != "GET" {
			t.Errorf("Expected GET request, go %s", r.Method)
		}
		expectedURL := fmt.Sprintf("/search/jobs/%s/messages", testSearchJob.ID)

		if r.URL.EscapedPath() != expectedURL {
			t.Errorf("Expected request to %s, got %s", expectedURL, r.URL.EscapedPath())
		}
		body, _ := json.Marshal(testSearchJobResult)
		w.Write(body)
	}))
	defer ts.Close()

	c, err := NewClient("accessToken", ts.URL)
	if err != nil {
		t.Errorf("error creating new client in search results test")
	}
	testSearchJob.apiClient = c
	returnedResults, err := testSearchJob.GetSearchResults(0, 2)
	if err != nil {
		t.Errorf("GetSearchJobResults() returned an error: %s", err)
	}
	if returnedResults == nil {
		t.Errorf("returnedResults nil")
	}

}
