package main

import "strings"

// WordsToInit is an array of eligible words for init reservation process
var WordsToInit = []string{"call the cab", "i need a ride", "ride now", "reserve ride", "แท็กซี่"}

// WordsToCancel is an array of eligible words for cancel reservation
var WordsToCancel = []string{"!reset", "reset", "cancel", "ยกเลิก"}

// WordsToAskForStatus is an array of eligible words for asking reservation status
var WordsToAskForStatus = []string{"!status", "status"}

// TargetPlaces is an array of eligible words for places in the service
var TargetPlaces = []string{"condo a", "citi resort", "bts phromphong"}

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
