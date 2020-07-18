package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// EventData struct
type EventData struct {
	Old Trip `json:"old"`
	New Trip `json:"new"`
}

// EventPayload struct
type EventPayload struct {
	SessionVariables map[string]json.RawMessage `json:"session_variables"`
	Op               string                     `json:"op"`
	Data             EventData                  `json:"data"`
}

// BodyPayload struct
type BodyPayload struct {
	Event        EventPayload               `json:"event"`
	CreatedAt    time.Time                  `json:"created_at"`
	ID           string                     `json:"id"`
	DeliveryInfo map[string]json.RawMessage `json:"delivery_info"`
	Trigger      map[string]json.RawMessage `json:"trigger"`
	Table        map[string]json.RawMessage `json:"table"`
}

// Response struct
type Response struct {
	Message string `json:"message"`
}

func jsonResponse(w http.ResponseWriter, httpCode int, resp Response) {
	w.WriteHeader(httpCode)
	w.Header().Set("Content-Type", "application/json")
	jMessage, _ := json.Marshal(resp)
	w.Write(jMessage)
}

// Webhook handles any request from Hasura server
func (app *HailingApp) Webhook(w http.ResponseWriter, req *http.Request) {
	method := req.Method
	if method != "POST" {
		errMessage := Response{Message: fmt.Sprintf("%s Method is not allowed", method)}
		jsonResponse(w, 400, errMessage)
		return
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var t BodyPayload
	err = json.Unmarshal(body, &t)
	if err != nil {
		log.Println("[WEBHOOK] Error JSON")
		errMessage := Response{Message: fmt.Sprintf("Wrong payload")}
		jsonResponse(w, 500, errMessage)
		return
	}
	log.Println("[WEBHOOK] Old: ", t.Event.Data.Old)
	log.Println("[WEBHOOK] New: ", t.Event.Data.New)
	log.Println("[WEBHOOK] Op: ", t.Event.Op)
	log.Println("[WEBHOOK] Trigger: ", t.Trigger)

	msg := Response{Message: "success"}
	jsonResponse(w, 200, msg)
}
