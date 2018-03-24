package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cleanshavenalex/sumologic-sdk-go"
)

func main() {
	if os.Getenv("SUMO_API_ACCESS_BASE64") == "" {
		fmt.Println("Must declare ENV var SUMO_API_ACCESS_BASE64 - base64 of access_id:access_key")
		os.Exit(1)
	}
	if os.Getenv("SUMO_API_HOST") == "" {
		fmt.Printf("Must declare ENV var SUMO_API_HOST \n https://help.sumologic.com/APIs/General-API-Information/Sumo-Logic-Endpoints-and-Firewall-Security\n")
		os.Exit(1)
	}
	if os.Getenv("SUMO_QUERY") == "" {
		fmt.Println("Must declare ENV var SUMO_QUERY")
		os.Exit(1)
	}
	sumoClient, err := sumologic.NewClient(os.Getenv("SUMO_API_ACCESS_BASE64"), os.Getenv("SUMO_API_HOST"))
	if err != nil {
		fmt.Printf("Error creating client : %v\n", err)
		os.Exit(1)
	}

	fmt.Println(sumoClient.EndpointURL)

	searchJob, err := sumoClient.StartSearch(sumologic.StartSearchRequest{
		Query:    os.Getenv("SUMO_QUERY"),
		From:     fmt.Sprintf(time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)),
		To:       fmt.Sprintf(time.Now().UTC().Format(time.RFC3339)),
		TimeZone: "PST",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	var jobState string

	// Poll the job state until it is done
	for jobState != sumologic.DoneGatheringResults {
		if jobState == sumologic.Canceled {
			fmt.Println(sumologic.Canceled)
			os.Exit(0)
		}
		if jobState == sumologic.ForcePaused {
			fmt.Println(sumologic.ForcePaused)
			os.Exit(0)
		}

		getJobStatus, err := searchJob.GetStatus()
		if err != nil {
			fmt.Printf("getJobStatus error: %v\n", err)
			os.Exit(1)
		}
		jobState = getJobStatus.State
		fmt.Printf("job state: %v \n", jobState)
		time.Sleep(2 * time.Second)
	}

	results, err := searchJob.GetSearchResults(0, 2)
	if err != nil {
		fmt.Printf("error getting results from finised search job %v, %v\n", searchJob.ID, err)
	}
	if results != nil {
		if results.Messages != nil {
			for k, v := range results.Messages {
				fmt.Printf("Message %v : %v\n", k, v)
			}
		}
	}

}
