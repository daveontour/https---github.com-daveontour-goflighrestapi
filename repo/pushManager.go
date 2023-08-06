package repo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron"

	"flightresourcerestapi/globals"
	"flightresourcerestapi/models"
	"flightresourcerestapi/timeservice"
)

func ReloadschedulePushes(airportCode string) {
	if _, ok := globals.SchedulerMap[airportCode]; ok {
		globals.SchedulerMap[airportCode].Clear()
		delete(globals.SchedulerMap, airportCode)
	}
	go SchedulePushes(airportCode)
}

var schedPushLock sync.Mutex

func SchedulePushes(airportCode string) {

	today := time.Now().Format("2006-01-02")
	s := gocron.NewScheduler(time.Local)

	globals.SchedulerMap[airportCode] = s

	for _, u := range globals.GetUserProfiles() {

		for _, sub := range u.UserPushSubscriptions {
			if sub.Airport != airportCode || !sub.Enabled {
				continue
			}

			startTimeStr := today + "T" + sub.Time
			startTime, _ := time.ParseInLocation("2006-01-02T15:04:05", startTimeStr, timeservice.Loc)

			if sub.ReptitionHours != 0 {
				user := u.UserName
				token := u.Key
				s.Every(sub.ReptitionHours).Hours().StartAt(startTime).Tag(token).Do(func() { executeScheduledPush(sub, token, user) })
				globals.Logger.Info(fmt.Sprintf("Scheduled Push for user %s, starting from %s, repeating every %v hours", u.UserName, startTimeStr, sub.ReptitionHours))
				if sub.PushOnStartUp {
					go executeScheduledPush(sub, token, user)
				}

			}
			if sub.ReptitionMinutes != 0 {
				user := u.UserName
				token := u.Key
				s.Every(sub.ReptitionMinutes).Minutes().StartAt(time.Now()).Tag(token).Do(func() { executeScheduledPush(sub, token, user) })
				globals.Logger.Info(fmt.Sprintf("Scheduled Push for user %s, starting from now, repeating every %v minutes", u.UserName, sub.ReptitionMinutes))
				if sub.PushOnStartUp {
					go executeScheduledPush(sub, token, user)
				}
			}
		}
	}

	s.StartBlocking()
}

func HandleFlightUpdate(flt models.Flight) {
	checkForImpactedSubscription(flt)
	return
}

func HandleFlightCreate(flt models.Flight) {
	checkForImpactedSubscription(flt)
	return
}

func HandleFlightDelete(flt models.Flight) {
	checkForImpactedSubscription(flt)
	return
}

// Check if any of the registered change subscriptions are interested in this change
func checkForImpactedSubscription(flt models.Flight) {

	sto := flt.GetSTO()

	if sto.Local().After(time.Now().Local().Add(36 * time.Hour)) {
		return
	}

	globals.UserChangeSubscriptionsMutex.Lock()
	defer globals.UserChangeSubscriptionsMutex.Unlock()

NextSub:
	for _, sub := range globals.UserChangeSubscriptions {

		if !sub.Enabled {
			continue
		}
		if sub.Airport != flt.GetIATAAirport() {
			continue
		}

		if !sub.UpdateFlight && flt.Action == globals.UpdateAction {
			continue
		}
		if sub.CreateFlight && flt.Action == globals.CreateAction {
			go executeChangePush(sub, flt)
			continue NextSub
		}
		if !sub.DeleteFlight && flt.Action == globals.DeleteAction {
			continue
		}

		if sub.DeleteFlight && flt.Action == globals.DeleteAction {
			go executeChangePush(sub, flt)
			continue NextSub
		}
		if sub.CreateFlight && flt.Action == globals.CreateAction {
			go executeChangePush(sub, flt)
			continue NextSub
		}
		if !sub.UpdateFlight && flt.Action == globals.UpdateAction {
			continue
		}
		// Required Parameter Field Changes
		for _, change := range flt.FlightChanges.Changes {

			if globals.Contains(sub.ParameterChange, change.PropertyName) {
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
			go executeChangePush(sub, flt)
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
			go executeChangePush(sub, flt)
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

	return
}

// Function that sends the flight to the defined endpoint
func executeChangePush(sub models.UserChangeSubscription, flight models.Flight) {

	globals.Logger.Debug(fmt.Sprintf("Executing Change Push for User "))

	queryBody, _ := json.Marshal(flight)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, sub.DestinationURL, bodyReader)
	if err != nil {
		globals.Logger.Error(fmt.Sprintf("Change Push Client: could not create change request: %s\n", err))
	}

	req.Header.Set("Content-Type", "application/json")
	for _, pair := range sub.HeaderParameters {
		req.Header.Add(pair.Parameter, pair.Value)
	}

	client := http.Client{
		Timeout: 20 * time.Second,
	}
	r, sendErr := client.Do(req)
	if sendErr != nil {
		globals.Logger.Error(fmt.Sprintf("Change Push Client. Error making http request: %s", sendErr))
	}
	if r.StatusCode != 200 {
		globals.Logger.Error(fmt.Sprintf("Change Push Client. Error making HTTP request: Returned status code = %v. URL = %s", r.StatusCode, sub.DestinationURL))
	}
}

func executeScheduledPush(sub models.UserPushSubscription, userToken, userName string) {

	// schedPushLock.Lock()
	// defer schedPushLock.Unlock()

	globals.Logger.Info(fmt.Sprintf("Executing Scheduled Push for User %s", userName))

	var response interface{}
	var error models.GetFlightsError

	// filterMutex.Lock()
	// defer filterMutex.Unlock()
	if strings.ToLower(sub.SubscriptionType) == "flight" {
		response, error = GetRequestedFlightsSub(sub, userToken)
	} else if strings.ToLower(sub.SubscriptionType) == "resource" {
		response, error = GetResourceSub(sub, userToken)
	}

	if error.Err != nil {
		globals.Logger.Error(fmt.Sprintf("Scheduled Push Client for user %s: Error with scheduled push %s", userName, error.Error()))
		return
	}

	queryBody, _ := json.Marshal(response)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, sub.DestinationURL, bodyReader)
	if err != nil {
		globals.Logger.Error(fmt.Sprintf("Scheduled Push Client for user %s: could not create request: %s\n", userName, err))
	}

	req.Header.Set("Content-Type", "application/json")
	for _, pair := range sub.HeaderParameters {
		req.Header.Add(pair.Parameter, pair.Value)
	}

	r, sendErr := http.DefaultClient.Do(req)
	if sendErr != nil {
		globals.Logger.Error(fmt.Sprintf("Scheduled Push Client for user %s: Error making http request: %s\n", userName, sendErr))
	}
	if r.StatusCode != 200 {
		globals.Logger.Error(fmt.Sprintf("Scheduled Push Client. Error making HTTP request: Returned status code = %v. URL = %s", r.StatusCode, sub.DestinationURL))
	}

	return
}
