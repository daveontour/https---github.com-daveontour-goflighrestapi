package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
)

func reloadschedulePushes(airportCode string) {
	if _, ok := schedulerMap[airportCode]; ok {
		schedulerMap[airportCode].Clear()
		delete(schedulerMap, airportCode)
	}
	go schedulePushes(airportCode)
}

func schedulePushes(airportCode string) {

	today := time.Now().Format("2006-01-02")
	s := gocron.NewScheduler(time.Local)

	schedulerMap[airportCode] = s

	for _, u := range getUserProfiles() {
		for _, sub := range u.UserPushSubscriptions {
			if sub.Airport != airportCode || !sub.Enabled {
				continue
			}

			startTimeStr := today + "T" + sub.Time
			startTime, _ := time.ParseInLocation("2006-01-02T15:04:05", startTimeStr, loc)

			if sub.ReptitionHours != 0 {
				user := u.UserName
				token := u.Key
				s.Every(sub.ReptitionHours).Hours().StartAt(startTime).Tag(token).Do(func() { executeScheduledPush(sub, token, user) })
				logger.Info(fmt.Sprintf("Scheduled Push for user %s, starting from %s, repeating every %v hours", u.UserName, startTimeStr, sub.ReptitionHours))
				if sub.PushOnStartUp {
					go executeScheduledPush(sub, token, user)
				}

			}
			if sub.ReptitionMinutes != 0 {
				user := u.UserName
				token := u.Key
				s.Every(sub.ReptitionMinutes).Minutes().StartAt(time.Now()).Tag(token).Do(func() { executeScheduledPush(sub, token, user) })
				logger.Info(fmt.Sprintf("Scheduled Push for user %s, starting from now, repeating every %v minutes", u.UserName, sub.ReptitionMinutes))
				if sub.PushOnStartUp {
					go executeScheduledPush(sub, token, user)
				}
			}
		}
	}

	s.StartBlocking()
}

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

		if !sub.UpdateFlight && flt.Action == UpdateAction {
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
		if sub.CreateFlight && flt.Action == CreateAction {
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

	r, sendErr := http.DefaultClient.Do(req)
	if sendErr != nil {
		logger.Error(fmt.Sprintf("Change Push Client. Error making http request: %s", sendErr))
	}
	if r.StatusCode != 200 {
		logger.Error(fmt.Sprintf("Change Push Client. Error making HTTP request: Returned status code = %v. URL = %s", r.StatusCode, sub.DestinationURL))
	}
}

func executeScheduledPush(sub UserPushSubscription, userToken, userName string) {

	logger.Info(fmt.Sprintf("Executing Scheduled Push for User %s", userName))

	var response interface{}
	var error GetFlightsError

	if strings.ToLower(sub.SubscriptionType) == "flight" {
		response, error = getRequestedFlightsSub(sub, userToken)
	} else if strings.ToLower(sub.SubscriptionType) == "resource" {
		response, error = getResourceSub(sub, userToken)
	}

	if error.Err != nil {
		logger.Error(fmt.Sprintf("Scheduled Push Client for user %s: Error with scheduled push %s", userName, error.Error()))
		return
	}

	queryBody, _ := json.Marshal(response)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, sub.DestinationURL, bodyReader)
	if err != nil {
		logger.Error(fmt.Sprintf("Scheduled Push Client for user %s: could not create request: %s\n", userName, err))
	}

	req.Header.Set("Content-Type", "application/json")
	for _, pair := range sub.HeaderParameters {
		req.Header.Add(pair.Parameter, pair.Value)
	}

	r, sendErr := http.DefaultClient.Do(req)
	if sendErr != nil {
		logger.Error(fmt.Sprintf("Scheduled Push Client for user %s: Error making http request: %s\n", userName, sendErr))
	}
	if r.StatusCode != 200 {
		logger.Error(fmt.Sprintf("Scheduled Push Client. Error making HTTP request: Returned status code = %v. URL = %s", r.StatusCode, sub.DestinationURL))
	}
}
