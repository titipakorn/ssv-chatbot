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
	Geometry   string  `json:"geometry"`
	Distance   float64 `json:"distance"`
	Duration   float64 `json:"duration"`
	WeightName string  `json:"weight_name"`
	Weight     float64 `json:"weight"`
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
