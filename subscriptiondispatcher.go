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

}

func handleFlightDelete(flt Flight) {

}

func getChangeType(flt Flight) []string {

	return []string{}
}

func checkForImpactedSubscription(flt Flight) {

	sto := flt.GetSTO()

	if sto.Local().After(time.Now().Local().Add(36 * time.Hour)) {
		return
	}

NextSub:
	for _, sub := range userChangeSubscriptions {

		if !sub.Enabled {
			continue
		}
		if sub.Airport != flt.GetIATAAirport() {
			continue
		}

		// Required Parameter Field Changes
		for _, change := range flt.FlightChanges.Changes {

			if contains(sub.ParameterChange, change.PropertyName) {
				go executeChangePush(sub, flt)
				break NextSub
			}

			if (change.PropertyName == "Stand" && sub.StandChange) ||
				(change.PropertyName == "Gate" && sub.GateChange) ||
				(change.PropertyName == "CheckInCounters" && sub.CheckInChange) ||
				(change.PropertyName == "Carousel" && sub.CarouselChange) ||
				(change.PropertyName == "Chute" && sub.ChuteChange) {

				go executeChangePush(sub, flt)
				break NextSub
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

func executeChangePush(sub UserChangeSubscription, flight Flight) {

	logger.Info(fmt.Sprintf("Executing Change Push for User "))

	var error GetFlightsError

	if error.Err != nil {
		logger.Error(fmt.Sprintf("Error with scheduled push %s", error.Error()))
		return
	}

	queryBody, _ := json.Marshal(flight)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, sub.DestinationURL, bodyReader)
	if err != nil {
		logger.Error(fmt.Sprintf("client: could not create change request: %s\n", err))
	}

	req.Header.Set("Content-Type", "application/json")
	for _, pair := range sub.HeaderParameters {
		req.Header.Add(pair.Parameter, pair.Value)
	}

	_, sendErr := http.DefaultClient.Do(req)
	if sendErr != nil {
		logger.Error(fmt.Sprintf("Change Push for user. Error making http request: %s\n", sendErr))
	}

}
