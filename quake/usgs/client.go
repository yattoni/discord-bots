package usgs

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type Client struct {
}

type QueryResults struct {
	Type     string `json:"type"`
	Metadata struct {
		Generated int64  `json:"generated"`
		URL       string `json:"url"`
		Title     string `json:"title"`
		Status    int    `json:"status"`
		API       string `json:"api"`
		Count     int    `json:"count"`
	} `json:"metadata"`
	Features []Feature `json:"features"`
	Bbox     []float64 `json:"bbox"`
}

type Feature struct {
	Type       string `json:"type"`
	Properties struct {
		Mag     float64     `json:"mag"`
		Place   string      `json:"place"`
		Time    int64       `json:"time"`
		Updated int64       `json:"updated"`
		Tz      interface{} `json:"tz"`
		URL     string      `json:"url"`
		Detail  string      `json:"detail"`
		Felt    interface{} `json:"felt"`
		Cdi     interface{} `json:"cdi"`
		Mmi     interface{} `json:"mmi"`
		Alert   interface{} `json:"alert"`
		Status  string      `json:"status"`
		Tsunami int         `json:"tsunami"`
		Sig     int         `json:"sig"`
		Net     string      `json:"net"`
		Code    string      `json:"code"`
		Ids     string      `json:"ids"`
		Sources string      `json:"sources"`
		Types   string      `json:"types"`
		Nst     interface{} `json:"nst"`
		Dmin    float64     `json:"dmin"`
		Rms     float64     `json:"rms"`
		Gap     float64     `json:"gap"`
		MagType string      `json:"magType"`
		Type    string      `json:"type"`
		Title   string      `json:"title"`
	} `json:"properties"`
	Geometry struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	ID string `json:"id"`
}

func (feature Feature) String() string {
	builder := strings.Builder{}

	properties := feature.Properties

	loc, _ := time.LoadLocation("America/Chicago")
	eventTime := time.UnixMilli(properties.Time).In(loc)
	minutesAgo := time.Since(eventTime).Minutes()

	builder.WriteString(fmt.Sprintf("Time: %s, %.0f minutes ago\n", eventTime, minutesAgo))
	builder.WriteString(fmt.Sprintf("Type: %s\n", properties.Type))
	builder.WriteString(fmt.Sprintf("Magnitude: %.1f %s\n", properties.Mag, properties.MagType))
	builder.WriteString(fmt.Sprintf("Location: %s\n", properties.Place))
	// builder.WriteString(fmt.Sprintf("Coordinates: [%.4f, %.4f]\n", feature.Geometry.Coordinates[0], feature.Geometry.Coordinates[1]))
	builder.WriteString(fmt.Sprintf("Link: %s\n", properties.URL))
	return builder.String()
}

//https://earthquake.usgs.gov/fdsnws/event/1/
func NewClient() *Client {
	return &Client{}
}

func (client *Client) FetchQuakes(startTime, endTime time.Time) QueryResults {
	timeFormat := "2006-01-02T15:04:05"
	url := fmt.Sprintf("https://earthquake.usgs.gov/fdsnws/event/1/query?format=geojson&starttime=%s&endtime=%s",
		startTime.Format(timeFormat), endTime.Format(timeFormat))

	log.Println("Url: ", url)

	resp := client.httpGet(url)

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Println("Error closing response ", err)
		}
	}()

	var queryResults QueryResults

	if err := json.NewDecoder(resp.Body).Decode(&queryResults); err != nil {
		log.Println(err)
	}

	return queryResults
}

func (client *Client) httpGet(url string) *http.Response {
	// Build the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("NewRequest: ", err)
		return nil
	}

	// For control over HTTP client headers,
	// redirect policy, and other settings,
	// create a Client
	// A Client is an HTTP client
	httpClient := &http.Client{}

	// Send the request via a client
	// Do sends an HTTP request and
	// returns an HTTP response
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatal("Do: ", err)
		return nil
	}
	return resp
}
