package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type body struct {
	Code      string     `json:"code"`
	Routes    []Route    `json:"routes"`
	Waypoints []waypoint `json:"waypoints"`
}

// Route provide basic route info
type Route struct {
	Geometry          string  `json:"geometry"`
	Distance          float64 `json:"distance"`
	Duration          float64 `json:"duration"`
	WeightName        string  `json:"weight_name"`
	Weight            float64 `json:"weight"`
	DurationInTraffic float64 `json:"duration_in_traffic"`
	Source            string
}

type waypoint struct {
	Hint     string     `json:"hint"`
	Distance float64    `json:"distance"`
	Name     string     `json:"name"`
	Point    [2]float64 `json:"location"`
}

// GetTravelTime return travel time in minute
func GetTravelTime(mode string, rec ReservationRecord) (*Route, error) {
	// check if we have enough data
	if rec.FromCoords == [2]float64{0, 0} || rec.ToCoords == [2]float64{0, 0} {
		return nil, errors.New("Not enough data to get travel time")
	}
	osrmBaseURL := os.Getenv("OSRM_BASE_URL")
	if osrmBaseURL == "" {
		osrmBaseURL = "https://nishi.10z.dev/route/v1"
	}
	od := fmt.Sprintf("%.8f,%.8f;%.8f,%.8f", rec.FromCoords[0], rec.FromCoords[1], rec.ToCoords[0], rec.ToCoords[1])
	reqURL := fmt.Sprintf("%s/%s/%s", osrmBaseURL, mode, od)
	log.Printf("[GetTravelTime] URL: %v", reqURL)
	httpResp, err := http.Get(reqURL)
	if err != nil {
		log.Fatal("Err: ", err)
		return nil, err
	}
	byteValue, _ := ioutil.ReadAll(httpResp.Body)
	// log.Printf("[GetTravelTime] result: %v", byteValue)
	var resp body
	json.Unmarshal(byteValue, &resp)
	return &resp.Routes[0], nil
}

type tV struct {
	Text  string  `json:"text"`
	Value float64 `json:"value"`
}

type latlon struct {
	Lat string `json:"lat"`
	Lon string `json:"lng"`
}

type ggLeg struct {
	Distance          tV     `json:"distance"`
	Duration          tV     `json:"duration"`
	DurationInTraffic tV     `json:"duration_in_traffic"`
	StartAddress      string `json:"start_address"`
	EndAddress        string `json:"end_address"`
	StartLocation     latlon `json:"start_location"`
	EndLocation       latlon `json:"end_location"`
}

type ggRouteResp struct {
	Bounds           string  `json:"bounds"`
	Legs             []ggLeg `json:"legs"`
	OverviewPolyline string  `json:"overview_polyline"`
	Summary          string  `json:"summary"`
}

type ggDirectionResp struct {
	GeoCodedWaypoint string        `json:"geocoded_waypoints"`
	Routes           []ggRouteResp `json:"routes"`
	Status           string        `json:"status"`
}

// GetGoogleTravelTime returns travel time from Google which has traffic info
func GetGoogleTravelTime(rec ReservationRecord) (*Route, error) {
	if rec.ReservedAt.Format("2006-01-02") == "0001-01-01" {
		// if there is no time, then we don't need to use Google API
		return GetTravelTime("car", rec)
	}
	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	if googleAPIKey == "" {
		return nil, errors.New("GOOGLE_API_KEY env: missing")
	}

	baseURL := "https://maps.googleapis.com/maps/api/directions/json?departure_time=%d&traffic_model=pessimistic&origin=%s&destination=%s&key=%s"
	departureTime := rec.ReservedAt.Unix()
	origin := fmt.Sprintf("%.8f,%.8f", rec.FromCoords[1], rec.FromCoords[0])
	dest := fmt.Sprintf("%.8f,%.8f", rec.ToCoords[1], rec.ToCoords[0])
	url := fmt.Sprintf(baseURL, departureTime, origin, dest, googleAPIKey)
	// log.Printf("[GetGoogleTravelTime] URL=%s\n", url)
	httpResp, err := http.Get(url)
	if err != nil {
		log.Fatal("Err: ", err)
		return nil, err
	}
	ggResp := ggDirectionResp{}
	byteValue, _ := ioutil.ReadAll(httpResp.Body)
	json.Unmarshal(byteValue, &ggResp)
	if ggResp.Status != "OK" {
		errMsg := fmt.Sprintf("Google API: %v", ggResp.Status)
		return nil, errors.New(errMsg)
	}
	result := Route{Source: "Google"}
	leg := ggResp.Routes[0].Legs[0]
	result.Distance = leg.Distance.Value
	result.Duration = leg.Duration.Value
	result.DurationInTraffic = leg.DurationInTraffic.Value
	return &result, nil
}
