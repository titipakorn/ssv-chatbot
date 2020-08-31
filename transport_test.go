package main

import (
	"log"
	"testing"
	"time"
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
	log.Printf("Walk Time: %4.0f m /walkRoute%4.0f s", walkRoute.Distance, walkRoute.Duration)
	log.Printf(" > Polyline: %s\n", walkRoute.Geometry)

	carRoute, err := GetTravelTime("car", rec)
	if err != nil {
		t.Errorf("walk error: %v", err)
	}
	log.Printf(" Car Time: %4.0f m / %4.0f s", carRoute.Distance, carRoute.Duration)
	log.Printf(" > Polyline: %s\n", carRoute.Geometry)

	if walkRoute.Duration == carRoute.Duration {
		t.Errorf("IMPOSSIBLE: Car time == walk time")
	}
}

func TestGoogleTravelTimeService(t *testing.T) {
	rec := ReservationRecord{
		ReservedAt: time.Now(),
		FromCoords: [2]float64{100.5685933, 13.7319484},
		ToCoords:   [2]float64{100.5695537, 13.7430816},
	}
	route, err := GetGoogleTravelTime(rec)
	if err != nil {
		t.Errorf("GetGoogleTravelTime error: %v", err)
	}
	log.Printf("Google (car as default)\n")
	log.Printf("  Distance: %6.0f m\n", route.Distance)
	log.Printf("  Duration: %6.0f s\n", route.Duration)
	log.Printf("  Duration: %6.0f s in traffic\n", route.DurationInTraffic)
	log.Printf("  Polyline: %s\n", route.Geometry)

}
