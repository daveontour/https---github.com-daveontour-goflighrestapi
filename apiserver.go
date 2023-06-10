package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Flights []Flight
}

type UserProfile struct {
	AllowedAirlines       []string
	AllowedCustomFields   []string
	QueryableCustomFields []string
}

type Request struct {
	Direction   string
	Airline     string
	From        string
	To          string
	UserProfile UserProfile
}

func startGinServer() {

	router := gin.Default()

	router.GET("/getFlights/:apt", getRequestedFlights)
	router.Run()

}

func getRequestedFlights(c *gin.Context) {

	apt := c.Param("apt")
	direction := c.Query("type")
	airline := c.Query("al")
	from := c.Query("from")
	to := c.Query("to")

	userProfile := UserProfile{}
	userProfile.AllowedAirlines = []string{"QF"}
	userProfile.AllowedCustomFields = []string{"FlightUniqueID", "SYS_ETA", "de--_ActualArrival_Source00"}

	request := Request{Direction: direction, Airline: airline, From: from, To: to, UserProfile: userProfile}

	_, ok := repoMap[apt]

	if ok {
		flights := repoMap[apt].Flights
		response, err := filterFlights(request, flights, c)
		c.Writer.Header().Set("Content-Type", "application/json")

		if err == nil {
			c.JSON(http.StatusOK, response)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"Error": fmt.Sprintf("%s", err.Error())})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Airport %s not found", apt)})
	}
}

func filterFlights(request Request, flights map[string]Flight, c *gin.Context) (Response, error) {

	returnFlights := []Flight{}

	var from time.Time
	var to time.Time

	if request.From != "" {
		f, err := time.Parse("2006-01-02T15:04:05", request.From)
		if err != nil {
			return Response{}, err

		} else {
			from = f
		}
	}
	if (request.To) != "" {
		t, err := time.Parse("2006-01-02T15:04:05", request.To)
		if err != nil {
			return Response{}, err

		} else {
			to = t
		}
	}

	for _, f := range flights {

		if request.Direction == "DEP" && f.IsArrival() {
			continue
		}
		if request.Direction == "ARR" && !f.IsArrival() {
			continue
		}
		if request.Airline != "" && f.GetIATAAirline() != request.Airline {
			continue
		}

		if request.From != "" {
			if f.GetSTO().Before(from) {
				continue
			}
		}
		if request.To != "" {
			if f.GetSTO().After(to) {
				continue
			}
		}

		if request.UserProfile.AllowedAirlines != nil {
			if !contains(request.UserProfile.AllowedAirlines, f.GetIATAAirline()) {
				continue
			}
		}

		// if request.UserProfile.QueryableCustomFields != nil{
		// 	qMap := c.Request.URL.Query()
		// 	for _,qName:= range request.UserProfile.QueryableCustomFields {
		// 		 qValue, ok := qMap[qName]
		// 		if ok {
		// 			if f.GetProperty(qName) != qValue {

		// 			}
		// 		}
		// 	}
		// }

		returnFlights = append(returnFlights, prune(f, request))
	}
	return Response{Flights: returnFlights}, nil
}

func prune(flight Flight, request Request) Flight {

	flDup := flight.DuplicateFlight()

	properties := make(map[string]string)

	for _, p := range flDup.FlightState.Value {
		properties[p.PropertyName] = p.Text
	}

	flDup.FlightState.Value = []Value{}

	for _, property := range request.UserProfile.AllowedCustomFields {
		data, ok := properties[property]

		if ok {
			flDup.FlightState.Value = append(flDup.FlightState.Value, Value{property, data})
		}
	}

	changes := []Change{}

	for ii := 0; ii < len(flDup.FlightChanges.Changes); ii++ {
		ok := contains(request.UserProfile.AllowedCustomFields, flDup.FlightChanges.Changes[ii].PropertyName)
		if ok {
			changes = append(changes, flDup.FlightChanges.Changes[ii])
		}
	}

	flDup.FlightChanges.Changes = changes

	return flDup
}
