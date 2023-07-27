package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func handleFlightUpdate(flt Flight) {
	checkForImpactedSubscription(flt)
}

func handleFlightCreate(flt Flight) {
	checkForImpactedSubscription(flt)
}

func handleFlightDelete(flt Flight) {
	checkForImpactedSubscription(flt)
}

// Check if any of the registered change subscriptions are interested in this change
func checkForImpactedSubscription(flt Flight) {

	sto := flt.GetSTO()

	if sto.Local().After(time.Now().Local().Add(36 * time.Hour)) {
		return
	}

	changeSubscriptionMutex.Lock()
	defer changeSubscriptionMutex.Unlock()

NextSub:
	for _, sub := range userChangeSubscriptions {

		if !sub.Enabled {
			continue
		}
		if sub.Airport != flt.GetIATAAirport() {
			continue
		}
		if !sub.CreateFlight && flt.Action == UpdateAction {
			continue
		}
		if sub.CreateFlight && flt.Action == CreateAction {
			go executeChangePush(sub, flt)
			continue NextSub
		}
		if !sub.DeleteFlight && flt.Action == DeleteAction {
			continue
		}
		if sub.DeleteFlight && flt.Action == DeleteAction {
			go executeChangePush(sub, flt)
			continue NextSub
		}
		if !sub.UpdateFlight && flt.Action == UpdateAction {
			continue
		}
		// Required Parameter Field Changes
		for _, change := range flt.FlightChanges.Changes {

			if contains(sub.ParameterChange, change.PropertyName) {
				go executeChangePush(sub, flt)
				continue NextSub
			}

			if (change.PropertyName == "Stand" && sub.StandChange) ||
				(change.PropertyName == "Gate" && sub.GateChange) ||
				(change.PropertyName == "CheckInCounters" && sub.CheckInChange) ||
				(change.PropertyName == "Carousel" && sub.CarouselChange) ||
				(change.PropertyName == "Chute" && sub.ChuteChange) {

				go executeChangePush(sub, flt)
				continue NextSub
			}

		}

		if sub.CheckInChange && flt.FlightChanges.CheckinSlotsChange != nil {
			go executeChangePush(sub, flt)
			continue
		}
		if sub.GateChange && flt.FlightChanges.GateSlotsChange != nil {
			executeChangePush(sub, flt)
			continue
		}
		if sub.StandChange && flt.FlightChanges.StandSlotsChange != nil {
			go executeChangePush(sub, flt)
			continue
		}
		if sub.ChuteChange && flt.FlightChanges.ChuteSlotsChange != nil {
			go executeChangePush(sub, flt)
			continue
		}
		if sub.CarouselChange && flt.FlightChanges.CarouselSlotsChange != nil {
			executeChangePush(sub, flt)
			continue
		}

		if sub.AircraftTypeOrRegoChange && flt.FlightChanges.AircraftTypeChange != nil {
			go executeChangePush(sub, flt)
			continue
		}
		if sub.AircraftTypeOrRegoChange && flt.FlightChanges.AircraftChange != nil {
			go executeChangePush(sub, flt)
			continue
		}
	}
}

// Function that sends the flight to the defined endpoint
func executeChangePush(sub UserChangeSubscription, flight Flight) {

	logger.Debug(fmt.Sprintf("Executing Change Push for User "))

	queryBody, _ := json.Marshal(flight)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, sub.DestinationURL, bodyReader)
	if err != nil {
		logger.Error(fmt.Sprintf("Change Push Client: could not create change request: %s\n", err))
	}

	req.Header.Set("Content-Type", "application/json")
	for _, pair := range sub.HeaderParameters {
		req.Header.Add(pair.Parameter, pair.Value)
	}

	_, sendErr := http.DefaultClient.Do(req)
	if sendErr != nil {
		logger.Error(fmt.Sprintf("Change Push Client. Error making http request: %s\n", sendErr))
	}

}
