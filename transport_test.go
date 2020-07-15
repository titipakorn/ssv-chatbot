package main

import (
	"log"
	"testing"
)

func TestTravelTimeService(t *testing.T) {
	rec := ReservationRecord{
		FromCoords: [2]float64{100.5685933, 13.7319484},
		ToCoords:   [2]float64{100.5695537, 13.7430816},
	}
	walkRoute, err := GetTravelTime("walk", rec)
	if err != nil {
		t.Errorf("walk error: %v", err)
	}
	log.Printf("Walk Time: %4.0f m / %4.0f s", walkRoute.Distance, walkRoute.Duration)

	carRoute, err := GetTravelTime("car", rec)
	if err != nil {
		t.Errorf("walk error: %v", err)
	}
	log.Printf(" Car Time: %4.0f m / %4.0f s", carRoute.Distance, carRoute.Duration)

	if walkRoute.Duration == carRoute.Duration {
		t.Errorf("IMPOSSIBLE: Car time == walk time")
	}
}
