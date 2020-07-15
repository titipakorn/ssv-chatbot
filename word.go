package main

import (
	"strings"
)

// WordsToInit is an array of eligible words for init reservation process
var WordsToInit = []string{"call the cab", "i need a ride", "ride now", "reserve ride", "init", "แท็กซี่"}

// WordsToCancel is an array of eligible words for cancel reservation
var WordsToCancel = []string{"!reset", "reset", "cancel", "ยกเลิก", "start over"}

// WordsToAskForStatus is an array of eligible words for asking reservation status
var WordsToAskForStatus = []string{"!status", "status"}

// TargetPlaces is an array of eligible words for places in the service
var TargetPlaces = []string{"condo a", "citi resort", "bts phromphong"}

// TargetPlaceCoords is a list of coords for places in the service
var TargetPlaceCoords = [][2]float64{
	{100.5623, 13.7349},
	{100.5749098, 13.7354784},
	{100.5698, 13.7304}}

// IsThisIn is an exactly "key in list"
func IsThisIn(word string, groupsOfWords []string) bool {
	lowercase := strings.ToLower(word)
	for _, a := range groupsOfWords {
		if a == lowercase {
			return true
		}
	}
	return false
}

func indexOf(word string, data []string) int {
	lower := strings.ToLower(word)
	for k, v := range data {
		if lower == v {
			return k
		}
	}
	return -1
}

// GetCoordsFromPlace return coords from place name
func GetCoordsFromPlace(place string) [2]float64 {
	mInd := indexOf(place, TargetPlaces)
	if mInd == -1 {
		return [2]float64{0, 0}
	}
	return TargetPlaceCoords[mInd]
}
