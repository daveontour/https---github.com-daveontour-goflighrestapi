package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
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

	loc, _ := time.LoadLocation("Local")
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
				s.Every(sub.ReptitionHours).Hours().StartAt(startTime).Tag(token).Do(func() { executePush(sub, token, user) })
				logger.Info(fmt.Sprintf("Scheduled Push for user %s, starting from %s, repeating every %v hours", u.UserName, startTimeStr, sub.ReptitionHours))
				if sub.PushOnStartUp {
					go executePush(sub, token, user)
				}

			}
			if sub.ReptitionMinutes != 0 {
				user := u.UserName
				token := u.Key
				s.Every(sub.ReptitionMinutes).Minutes().StartAt(time.Now()).Tag(token).Do(func() { executePush(sub, token, user) })
				logger.Info(fmt.Sprintf("Scheduled Push for user %s, starting from now, repeating every %v minutes", u.UserName, sub.ReptitionMinutes))
				if sub.PushOnStartUp {
					go executePush(sub, token, user)
				}
			}
		}
	}

	s.StartBlocking()
}

func executePush(sub UserPushSubscription, userToken, userName string) {

	logger.Info(fmt.Sprintf("Executing Scheduled Push for User %s", userName))

	var response interface{}
	var error GetFlightsError

	if strings.ToLower(sub.SubscriptionType) == "flight" {
		response, error = getRequestedFlightsSub(sub, userToken)
	} else if strings.ToLower(sub.SubscriptionType) == "resource" {
		response, error = getResourceSub(sub, userToken)
	}

	if error.Err != nil {
		logger.Error(fmt.Sprintf("Error with scheduled push %s", error.Error()))
		return
	}

	queryBody, _ := json.Marshal(response)
	bodyReader := bytes.NewReader([]byte(queryBody))

	req, err := http.NewRequest(http.MethodPost, sub.DestinationURL, bodyReader)
	if err != nil {
		logger.Error(fmt.Sprintf("client: could not create request: %s\n", err))
	}

	req.Header.Set("Content-Type", "application/json")
	for _, pair := range sub.HeaderParameters {
		req.Header.Add(pair.Parameter, pair.Value)
	}

	_, sendErr := http.DefaultClient.Do(req)
	if sendErr != nil {
		logger.Error(fmt.Sprintf("Scheduled Push for user %s: Error making http request: %s\n", userName, sendErr))
	}

}

func getUserProfiles() []UserProfile {

	//Read read the file for each request so changes can be made without the need to restart the server

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	fileContent, err := os.Open(filepath.Join(exPath, "users.json"))

	if err != nil {
		logger.Error("Error Reading " + filepath.Join(exPath, "users.json"))
		logger.Error(err.Error())
		return []UserProfile{}
	}

	defer fileContent.Close()

	byteResult, _ := ioutil.ReadAll(fileContent)

	var users Users

	json.Unmarshal([]byte(byteResult), &users)

	return users.Users
}
