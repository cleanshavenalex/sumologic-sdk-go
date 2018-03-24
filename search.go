package sumologic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/juju/errors"
)

// https://help.sumologic.com/APIs/Search-Job-API/About-the-Search-Job-API#Creating_a_search_job
// TLDR;
// Rate Limit 240 rpm
// use ISO 8601 for time ranges
// Process Flow
// 1. Request a Search Job - Client.StartSearch(*Search) - query and time range.
// 2. Response - a search job ID or error SearchJob
// 3. Request search status - Client.GetSearchStatus(id int) must be done every 5 in at least
// 4. Response
//      - a job status 'gathering results', 'done executing', etc
//      - message and record counts
// 5. Request - request the results, job does not have to be complete
// 6. Response - JSON search results

// StartSearchRequest is the data needed to start a search
type StartSearchRequest struct {
	Query    string `json:"query"`
	From     string `json:"from"`
	To       string `json:"to"`
	TimeZone string `json:"timeZone"`
}

// SearchJob represents a search job in Sumologic, returned after starting a search.
type SearchJob struct {
	Status    int    `json:"status"`
	ID        string `json:"id,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	apiClient *Client
	cookies   []*http.Cookie
}

// ErrSearchJobNotFound is returned when the Search Job ID passed was not found
var ErrSearchJobNotFound = errors.New("Job ID is invalid")

// The different states a search job could be in.
const (
	NotStarted           = "NOT STARTED"
	GatheringResults     = "GATHERING RESULTS"
	ForcePaused          = "FORCED PAUSED"
	DoneGatheringResults = "DONE GATHERING RESULTS"
	Canceled             = "CANCELED"
)

// StartSearch calls the Sumologic API Search Endpoint.
// POST search/jobs
func (c *Client) StartSearch(ssr StartSearchRequest) (*SearchJob, error) {
	body, err := json.Marshal(ssr)
	if err != nil {
		return nil, errors.Annotate(err, "failed to create post body")
	}
	relativeURL, _ := url.Parse("search/jobs")
	url := c.EndpointURL.ResolveReference(relativeURL)

	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(body))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Basic "+c.AuthToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Annotate(err, "StartSearch request failed")
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Annotate(err, "failed to read resp.Body")
	}
	switch resp.StatusCode {
	case http.StatusAccepted:
		sj := &SearchJob{}
		err = json.Unmarshal(responseBody, &sj)
		if err != nil {
			return nil, errors.Annotate(err, "failed to parse start search response")
		}
		sj.cookies = resp.Cookies()
		sj.apiClient = c
		return sj, nil
	case http.StatusUnauthorized:
		return nil, ErrClientAuthenticationError
	case http.StatusBadRequest:
		sj := SearchJob{}
		err = json.Unmarshal(responseBody, &sj)
		if err != nil {
			return nil, errors.Annotate(err, "failed to parse bad request response")
		}
		return nil, errors.Annotatef(err, "Start SearchJob BadRequest, %v, %v", sj.Code, sj.Message)
	default:
		return nil, errors.Annotatef(err, "unexepected http status code %v", resp.StatusCode)
	}
}

// HistogramBucket corresponds to the histogram display in the Sumo Logic interactive analytics API.
type HistogramBucket struct {
	Length         int `json:"length"`
	Count          int `json:"count"`
	StartTimeStamp int `json:"startTimeStamp"`
}

// SearchJobStatusResponse stores the response from getting a search status.
type SearchJobStatusResponse struct {
	State           string             `json:"state"`
	MessageCount    uint               `json:"messageCount"`
	HistgramBuckets []*HistogramBucket `json:"histogramBuckets"`
	RecordCount     uint               `json:"recordCount"`
	PendingWarnings []string           `json:"pendingWarnings"`
	PendingErrors   []string           `json:"pendingErrors"`
}

// GetStatus retrieves the status of a running job.
func (sj *SearchJob) GetStatus() (*SearchJobStatusResponse, error) {

	relativeURL, _ := url.Parse(fmt.Sprintf("search/jobs/%s", sj.ID))
	url := sj.apiClient.EndpointURL.ResolveReference(relativeURL)
	req, err := http.NewRequest("GET", url.String(), nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Basic "+sj.apiClient.AuthToken)
	for _, v := range sj.cookies {
		req.AddCookie(v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Annotate(err, "GetSearchJobStatus api request failed")
	}
	defer resp.Body.Close()

	responseBody, _ := ioutil.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		var jobStatus = new(SearchJobStatusResponse)
		err = json.Unmarshal(responseBody, &jobStatus)
		if err != nil {
			return nil, errors.Annotate(err, "GetSearchJobStatus failed to parse response")
		}
		return jobStatus, nil
	case http.StatusNotFound:
		return nil, ErrSearchJobNotFound
	default:
		return nil, errors.Annotatef(err, "GetSearchJobStatus response status not OK : %v", resp.StatusCode)
	}
}

// SearchJobResultsRequest is a wrapper for the search job messages params.
type SearchJobResultsRequest struct {
	ID     string `json:"searchJobId"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

// SearchJobResultField is one field from a search result.
type SearchJobResultField struct {
	Name      string `json:"name"`
	FieldType string `json:"fieldType"`
	KeyField  bool   `json:"keyField"`
}

// SearchJobResultMessage represents one message from a search job result.
type SearchJobResultMessage struct {
	Map []byte `json:"map"`
}

// SearchJobResult represents a search job result
type SearchJobResult struct {
	Fields   []SearchJobResultField   `json:"fields"`
	Messages []SearchJobResultMessage `json:"messages"`
}

// GetSearchResults will retrieve the messages from a finished search job.
func (sj *SearchJob) GetSearchResults(offset int, limit int) (*SearchJobResult, error) {
	relativeURL, err := url.Parse(fmt.Sprintf("search/jobs/%s/messages", sj.ID))
	if err != nil {
		return nil, errors.Annotatef(err, "failed to create relativeURL from ID : %v", sj.ID)
	}
	url := sj.apiClient.EndpointURL.ResolveReference(relativeURL)
	req, err := http.NewRequest("GET", url.String(), nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Basic "+sj.apiClient.AuthToken)
	for _, v := range sj.cookies {
		req.AddCookie(v)
	}
	q := req.URL.Query()
	// TODO add more tests
	// check sumo api response if offset not defined and add test case
	// same for limit
	q.Add("offset", strconv.Itoa(offset))
	q.Add("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Annotate(err, "GetSearchResults request to get search job messages failed")
	}
	defer resp.Body.Close()

	responseBody, _ := ioutil.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		var searchResult = new(SearchJobResult)
		err = json.Unmarshal(responseBody, &searchResult)
		if err != nil {
			return nil, errors.Annotate(err, "GetSearchResults failed to parse successful response")
		}
		return searchResult, nil
	default:
		return nil, errors.Annotatef(err, "Status not OK : %v", resp.StatusCode)
	}

}
