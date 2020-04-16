// Copyright 2016 LINE Corporation
//
// LINE Corporation licenses this file to you under the Apache License,
// version 2.0 (the "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at:
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {

	app, err := NewHailingApp(
		os.Getenv("CHANNEL_SECRET"),
		os.Getenv("CHANNEL_TOKEN"),
		os.Getenv("APP_BASE_URL"),
	)
	if err != nil {
		log.Fatal(err)
	}
	userID := "sipp11"
	reserveRec, err := app.FindRecord(userID)
	if err != nil {
		fmt.Println("reserve err: ", err)
	}
	fmt.Println("reserve status: ", reserveRec)
	nextState := reserveRec.WhatsNext()
	fmt.Println("NEXT state: ", nextState)
	if nextState == "to" {
		// set to next state and wait for reply
		reserveRec.State = nextState
		buff, _ := json.Marshal(&reserveRec)
		if err := app.rdb.Set(userID, buff, 5*time.Minute).Err(); err != nil {
			fmt.Println(" >> err: update state: ", err)
		}
		curr, err := app.ProcessReservationStep(userID, Reply{Text: "CITI Resort"})
		if err != nil {
			fmt.Println(" >> to err: ", err)
		}
		fmt.Println(" >> current state: ", curr)
	} else if nextState == "from" {
		// set to next state and wait for reply
		reserveRec.State = nextState
		buff, _ := json.Marshal(&reserveRec)
		if err := app.rdb.Set(userID, buff, 4*time.Minute).Err(); err != nil {
			fmt.Println(" >> err: update state: ", err)
		}
		curr, err := app.ProcessReservationStep(userID, Reply{Text: "BTS A"})
		if err != nil {
			fmt.Println(" >> to err: ", err)
		}
		fmt.Println(" >> current state: ", curr)
	} else if nextState == "when" {
		// set to next state and wait for reply
		reserveRec.State = nextState
		buff, _ := json.Marshal(&reserveRec)
		if err := app.rdb.Set(userID, buff, 3*time.Minute).Err(); err != nil {
			fmt.Println(" >> err: update state: ", err)
		}
		curr, err := app.ProcessReservationStep(userID, Reply{Text: "2020-04-15T19:00:00+07:00"})
		if err != nil {
			fmt.Println(" >> to err: ", err)
		}
		fmt.Println(" >> current state: ", curr)
	}

	// serve /static/** files
	staticFileServer := http.FileServer(http.Dir("static"))
	http.HandleFunc("/static/", http.StripPrefix("/static/", staticFileServer).ServeHTTP)
	// serve /downloaded/** files
	downloadedFileServer := http.FileServer(http.Dir(app.downloadDir))
	http.HandleFunc("/downloaded/", http.StripPrefix("/downloaded/", downloadedFileServer).ServeHTTP)

	http.HandleFunc("/callback", app.Callback)
	// This is just a sample code.
	// For actually use, you must support HTTPS by using `ListenAndServeTLS`, reverse proxy or etc.
	if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
		log.Fatal(err)
	}

}
