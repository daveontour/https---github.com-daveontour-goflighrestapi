package models

import (
	"encoding/json"
	"encoding/xml"
	"flightresourcerestapi/timeservice"
	"fmt"
	"log"
	"strings"
	"time"
)

type AirlineDesignator struct {
	CodeContext string `xml:"codeContext,attr"`
	Text        string `xml:",chardata"`
}

type AirportCode struct {
	CodeContext string `xml:"codeContext,attr"`
	Text        string `xml:",chardata"`
}

type FlightId struct {
	FlightKind        string              `xml:"FlightKind"`
	AirlineDesignator []AirlineDesignator `xml:"AirlineDesignator"`
	FlightNumber      string              `xml:"FlightNumber"`
	ScheduledDate     string              `xml:"ScheduledDate"`
	AirportCode       []AirportCode       `xml:"AirportCode"`
}

func (d FlightId) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("{")

	fk, _ := json.Marshal(d.FlightKind)
	sb.WriteString(fmt.Sprintf("\"FlightKind\":%s,", string(fk)))

	fn, _ := json.Marshal(d.FlightNumber)
	sb.WriteString(fmt.Sprintf("\"FlightNumber\":%s,", string(fn)))

	sd, _ := json.Marshal(d.ScheduledDate)
	sb.WriteString(fmt.Sprintf("\"ScheduledDate\":%s,", string(sd)))

	if d.AirportCode != nil {
		sb.WriteString("\"AirportCode\":{")

		for idx, apt := range d.AirportCode {
			if idx > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", apt.CodeContext, apt.Text))
		}
	}
	sb.WriteString("}")

	if d.AirlineDesignator != nil {
		sb.WriteString(",\"AirlineDesignator\":{")

		for idx, al := range d.AirlineDesignator {
			if idx > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", al.CodeContext, al.Text))
		}
	}

	sb.WriteString("}")
	sb.WriteString("}")

	return []byte(sb.String()), nil
}

type Value struct {
	PropertyName string `xml:"propertyName,attr"`
	Text         string `xml:",chardata"`
}

func (d Value) MarshalJSON() ([]byte, error) {
	v := fmt.Sprintf("{\"%s\":\"%s\"}", d.PropertyName, d.Text)
	return []byte(v), nil
}

type LinkedFlight struct {
	FlightId FlightId `xml:"FlightId"`
	Value    []Value  `xml:"Value"`
}

func (d LinkedFlight) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("{")

	fid, _ := json.Marshal(d.FlightId)
	if fid == nil {
		sb.WriteString("}")
		return []byte(sb.String()), nil
	}
	sb.WriteString(fmt.Sprintf("\"FlightId\":%s,", string(fid)))

	vs := MarshalJSON(d.Value)
	sb.WriteString(fmt.Sprintf("\"Values\":%s", string(vs)))

	s := CleanJSON(sb)

	return []byte(s), nil
}

type AircraftTypeCode struct {
	CodeContext string `xml:"codeContext,attr"`
	Text        string `xml:",chardata"`
}
type AircraftTypeId struct {
	//	Text             string             `xml:",chardata" json:"-"`
	AircraftTypeCode []AircraftTypeCode `xml:"AircraftTypeCode"`
}

func (tid AircraftTypeId) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("{")

	if tid.AircraftTypeCode != nil {
		sb.WriteString("\"AircraftTypeCode\":{")

		for _, tc := range tid.AircraftTypeCode {
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\",", tc.CodeContext, tc.Text))
		}
	}

	s := CleanJSON(sb)

	s = s + "}"

	return []byte(s), nil
}

type AircraftType struct {
	AircraftTypeId AircraftTypeId `xml:"AircraftTypeId"`
	Value          []Value        `xml:"Value"`
}

func (t AircraftType) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("{")

	tid, _ := json.Marshal(t.AircraftTypeId)
	sb.WriteString(fmt.Sprintf("\"AircraftTypeId\":%s,", string(tid)))

	vs := MarshalJSON(t.Value)
	sb.WriteString(fmt.Sprintf("\"Values\":%s", string(vs)))

	s := CleanJSON(sb)

	return []byte(s), nil
}

type RouteViaPoint struct {
	SequenceNumber string        `xml:"sequenceNumber,attr"`
	AirportCode    []AirportCode `xml:"AirportCode"`
}

type ViaPoints struct {
	RouteViaPoint []RouteViaPoint `xml:"RouteViaPoint"`
}

func (r ViaPoints) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("[")

	for idx, rvp := range r.RouteViaPoint {
		if idx > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("{")

		sb.WriteString(fmt.Sprintf("\"SequenceNumber\":\"%s\",", rvp.SequenceNumber))

		sb.WriteString("\"AirportCode\":{")

		for idx2, apt := range rvp.AirportCode {
			if idx2 > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", apt.CodeContext, apt.Text))
		}

		sb.WriteString("}")
		sb.WriteString("}")
	}

	sb.WriteString("]")

	return []byte(sb.String()), nil
}

type Route struct {
	CustomsType string    `xml:"customsType,attr"`
	ViaPoints   ViaPoints `xml:"ViaPoints"`
}

type TableValue struct {
	PropertyName string  `xml:"propertyName,attr"`
	Value        []Value `xml:"Value"`
}

func (ss StandSlots) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("[")

	for idx2, s := range ss.StandSlot {

		if idx2 > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("{")
		for idx3, v := range s.Value {
			if idx3 > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", v.PropertyName, v.Text))
		}
		for _, v := range s.Stand.Value {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", v.PropertyName, v.Text))
		}

		for _, v := range s.Stand.Area.Value {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("\"Area%s\":\"%s\"", v.PropertyName, v.Text))
		}
		sb.WriteString("}")

	}

	sb.WriteString("]")

	return []byte(sb.String()), nil
}

func (ss CarouselSlots) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("[")

	for idx2, s := range ss.CarouselSlot {

		if idx2 > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("{")
		for idx3, v := range s.Value {
			if idx3 > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", v.PropertyName, v.Text))
		}
		for _, v := range s.Carousel.Value {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", v.PropertyName, v.Text))
		}

		for _, v := range s.Carousel.Area.Value {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("\"Area%s\":\"%s\"", v.PropertyName, v.Text))
		}
		sb.WriteString("}")

	}

	sb.WriteString("]")

	return []byte(sb.String()), nil
}

func (ss GateSlots) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("[")

	for idx2, s := range ss.GateSlot {

		if idx2 > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("{")
		for idx3, v := range s.Value {
			if idx3 > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", v.PropertyName, v.Text))
		}
		for _, v := range s.Gate.Value {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", v.PropertyName, v.Text))
		}

		for _, v := range s.Gate.Area.Value {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("\"Area%s\":\"%s\"", v.PropertyName, v.Text))
		}

		sb.WriteString("}")

	}

	sb.WriteString("]")

	return []byte(sb.String()), nil
}

func (ss CheckInSlots) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("[")

	for idx2, s := range ss.CheckInSlot {

		if idx2 > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("{")
		for idx3, v := range s.Value {
			if idx3 > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", v.PropertyName, v.Text))
		}
		for _, v := range s.CheckIn.Value {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", v.PropertyName, v.Text))
		}
		for _, v := range s.CheckIn.Area.Value {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("\"Area%s\":\"%s\"", v.PropertyName, v.Text))
		}
		sb.WriteString("}")
	}

	sb.WriteString("]")

	return []byte(sb.String()), nil
}

func (ss ChuteSlots) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("[")

	for idx2, s := range ss.ChuteSlot {

		if idx2 > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("{")
		for idx3, v := range s.Value {
			if idx3 > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", v.PropertyName, v.Text))
		}
		for _, v := range s.Chute.Value {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("\"%s\":\"%s\"", v.PropertyName, v.Text))
		}
		for _, v := range s.Chute.Area.Value {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("\"Area%s\":\"%s\"", v.PropertyName, v.Text))
		}
		sb.WriteString("}")

	}

	sb.WriteString("]")

	return []byte(sb.String()), nil
}

type AircraftId struct {
	Registration string `xml:"Registration" json:"Registration" `
}
type Aircraft struct {
	AircraftId AircraftId `xml:"AircraftId" json:"AircraftId"`
}

type FlightState struct {
	ScheduledTime string        `xml:"ScheduledTime" `
	LinkedFlight  LinkedFlight  `xml:"LinkedFlight"`
	AircraftType  AircraftType  `xml:"AircraftType"`
	Aircraft      Aircraft      `xml:"Aircraft" json:"Aircraft"`
	Route         Route         `xml:"Route" json:"-"`
	Value         []Value       `xml:"Value" json:"Values,omitempty"`
	TableValue    []TableValue  `xml:"TableValue" json:"TableValues,omitempty"`
	StandSlots    StandSlots    `xml:"StandSlots" json:"StandSlots,omitempty"`
	CarouselSlots CarouselSlots `xml:"CarouselSlots" json:"CarouselSlots,omitempty"`
	GateSlots     GateSlots     `xml:"GateSlots" json:"GateSlots,omitempty"`
	CheckInSlots  CheckInSlots  `xml:"CheckInSlots" json:"CheckInSlots,omitempty"`
	ChuteSlots    ChuteSlots    `xml:"ChuteSlots" json:"ChuteSlots,omitempty"`
}

func MarshalJSON(vs []Value) []byte {

	var sb strings.Builder

	sb.WriteString("{")

	for _, f := range vs {
		sb.WriteString(fmt.Sprintf("\"%s\":\"%s\",", f.PropertyName, f.Text))
	}

	s := sb.String()
	if last := len(s) - 1; last >= 0 && s[last] == ',' {
		s = s[:last]
	}

	s = strings.Replace(s, "\n", "", -1)

	s = s + "}"

	return []byte(s)
}
func (d FlightState) MarshalJSON() ([]byte, error) {

	var sb strings.Builder
	sb.WriteString("{")

	st, _ := json.Marshal(d.ScheduledTime)
	sb.WriteString(fmt.Sprintf("\"ScheduledTime\":%s,", string(st)))

	lf, _ := json.Marshal(d.LinkedFlight)
	sb.WriteString(fmt.Sprintf("\"LinkedFlight\":%s,", string(lf)))

	ac, _ := json.Marshal(d.AircraftType)
	sb.WriteString(fmt.Sprintf("\"AircraftType\":%s,", string(ac)))

	ac2, _ := json.Marshal(d.Aircraft)
	sb.WriteString(fmt.Sprintf("\"Aircraft\":%s,", string(ac2)))

	rt, _ := json.Marshal(d.Route)
	sb.WriteString(fmt.Sprintf("\"Route\":%s,", string(rt)))

	vs := MarshalJSON(d.Value)
	sb.WriteString(fmt.Sprintf("\"Values\":%s,", string(vs)))

	ss, _ := json.Marshal(d.StandSlots)
	sb.WriteString(fmt.Sprintf("\"StandSlots\":%s,", string(ss)))

	cs, _ := json.Marshal(d.CarouselSlots)
	sb.WriteString(fmt.Sprintf("\"CarouselSlots\":%s,", string(cs)))

	gs, _ := json.Marshal(d.GateSlots)
	sb.WriteString(fmt.Sprintf("\"GateSlots\":%s,", string(gs)))

	cis, _ := json.Marshal(d.CheckInSlots)
	sb.WriteString(fmt.Sprintf("\"CheckInSlots\":%s,", string(cis)))

	chs, _ := json.Marshal(d.ChuteSlots)
	sb.WriteString(fmt.Sprintf("\"ChuteSlots\":%s,", string(chs)))

	s := CleanJSON(sb)

	return []byte(s), nil
}

type Change struct {
	PropertyName string `xml:"propertyName,attr"`
	OldValue     string `xml:"OldValue"`
	NewValue     string `xml:"NewValue"`
}

type GateSlotsChange struct {
	OldValue struct {
		GateSlot struct {
			Value []PropertyValuePair `xml:"Value"`
		} `xml:"GateSlot"`
	} `xml:"OldValue"`
	NewValue struct {
		GateSlot struct {
			Value []PropertyValuePair `xml:"Value"`
		} `xml:"GateSlot"`
	} `xml:"NewValue"`
}
type StandSlotChange struct {
	OldValue struct {
		StandSlot struct {
			Value []PropertyValuePair `xml:"Value"`
		} `xml:"StandSlot"`
	} `xml:"OldValue"`
	NewValue struct {
		StandSlot struct {
			Value []PropertyValuePair `xml:"Value"`
		} `xml:"StandSlot"`
	} `xml:"NewValue"`
}
type CheckInSlotsChange struct {
	OldValue struct {
		CheckInSlot []struct {
			Value   []PropertyValuePair `xml:"Value"`
			CheckIn struct {
				Value []PropertyValuePair `xml:"Value"`
				Area  struct {
					Value PropertyValuePair `xml:"Value"`
				} `xml:"Area"`
			} `xml:"CheckIn"`
		} `xml:"CheckInSlot"`
	} `xml:"OldValue"`
	NewValue struct {
		CheckInSlot []struct {
			Value   []PropertyValuePair `xml:"Value"`
			CheckIn struct {
				Value []PropertyValuePair `xml:"Value"`
				Area  struct {
					Value PropertyValuePair `xml:"Value"`
				} `xml:"Area"`
			} `xml:"CheckIn"`
		} `xml:"CheckInSlot"`
	} `xml:"NewValue"`
}
type CarouselSlotsChange struct {
	OldValue struct {
		CarouselSlot struct {
			Value []PropertyValuePair `xml:"Value"`
		} `xml:"CarouselSlot"`
	} `xml:"OldValue"`
	NewValue struct {
		CarouselSlot struct {
			Value    []PropertyValuePair `xml:"Value"`
			Carousel struct {
				Value []PropertyValuePair `xml:"Value"`
				Area  struct {
					Value struct {
						Text         string `xml:",chardata"`
						PropertyName string `xml:"propertyName,attr"`
					} `xml:"Value"`
				} `xml:"Area"`
			} `xml:"Carousel"`
		} `xml:"CarouselSlot"`
	} `xml:"NewValue"`
}
type ChuteSlotsChange struct {
	OldValue struct {
		ChuteSlot struct {
			Value []struct {
				Text         string `xml:",chardata"`
				PropertyName string `xml:"propertyName,attr"`
			} `xml:"Value"`
		} `xml:"ChuteSlot"`
	} `xml:"OldValue"`
	NewValue struct {
		ChuteSlot struct {
			Value PropertyValuePair `xml:"Value"`
			Chute struct {
				Value []PropertyValuePair `xml:"Value"`
				Area  struct {
					Value PropertyValuePair `xml:"Value"`
				} `xml:"Area"`
			} `xml:"Chute"`
		} `xml:"ChuteSlot"`
	} `xml:"NewValue"`
}
type AircraftTypeChange struct {
	OldValue struct {
		AircraftType struct {
			AircraftTypeId struct {
				AircraftTypeCode []struct {
					Text        string `xml:",chardata"`
					CodeContext string `xml:"codeContext,attr"`
				} `xml:"AircraftTypeCode"`
			} `xml:"AircraftTypeId"`
			Value PropertyValuePair `xml:"Value"`
		} `xml:"AircraftType"`
	} `xml:"OldValue"`
	NewValue struct {
		AircraftType struct {
			AircraftTypeId struct {
				AircraftTypeCode []struct {
					Text        string `xml:",chardata"`
					CodeContext string `xml:"codeContext,attr"`
				} `xml:"AircraftTypeCode"`
			} `xml:"AircraftTypeId"`
			Value PropertyValuePair `xml:"Value"`
		} `xml:"AircraftType"`
	} `xml:"NewValue"`
}
type AircraftChange struct {
	OLdValue struct {
		Aircraft struct {
			AircraftId struct {
				Registration string `xml:"Registration"`
			} `xml:"AircraftId"`
			Value PropertyValuePair `xml:"Value"`
		} `xml:"Aircraft"`
	} `xml:"OldValue"`
	NewValue struct {
		Aircraft struct {
			AircraftId struct {
				Registration string `xml:"Registration"`
			} `xml:"AircraftId"`
			Value PropertyValuePair `xml:"Value"`
		} `xml:"Aircraft"`
	} `xml:"NewValue"`
}
type FlightChanges struct {
	AircraftTypeChange  *AircraftTypeChange  `xml:"AircraftTypeChange" json:"AircraftTypeChange"`
	AircraftChange      *AircraftChange      `xml:"AircraftChange" json:"AircraftChange"`
	CarouselSlotsChange *CarouselSlotsChange `xml:"CarouselSlotsChange" json:"CarouselSlotsChange"`
	GateSlotsChange     *GateSlotsChange     `xml:"GateSlotsChange" json:"GateSlotsChange"`
	StandSlotsChange    *StandSlotChange     `xml:"StandSlotsChange" json:"StandSlotsChange"`
	ChuteSlotsChange    *ChuteSlotsChange    `xml:"ChuteSlotsChange" json:"ChuteSlotsChange"`
	CheckinSlotsChange  *CheckInSlotsChange  `xml:"CheckInSlotsChange" json:"CheckInSlotsChange"`
	Changes             []Change             `xml:"PropertyChanges"`
}
type Flight struct {
	PrevNode      *Flight       `xml:"-" json:"-"`
	NextNode      *Flight       `xml:"-" json:"-"`
	Action        string        `xml:"Action" json:"Action"`
	FlightId      FlightId      `xml:"FlightId" json:"FlightId"`
	FlightState   FlightState   `xml:"FlightState" json:"FlightState"`
	FlightChanges FlightChanges `xml:"FlightChanges" json:"FlightChanges"`
	LastUpdate    time.Time     `xml:"LastUpdate" json:"LastUpdate"`
}
type Flights struct {
	Flight []Flight `xml:"Flight" json:"Flights"`
}
type StandAllocation struct {
	Stand  Stand
	From   time.Time
	To     time.Time
	Flight FlightId
}

type StandAllocations struct {
	Allocations []StandAllocation
}

// Resource definitions

type Area struct {
	Value []Value `xml:"Value"`
}

type Stand struct {
	Value []Value `xml:"Value" json:"Slot,omitempty"`
	Area  Area    `xml:"Area" json:"Area,omitempty"`
}

type StandSlot struct {
	Value []Value `xml:"Value" json:"Slot,omitempty"`
	Stand Stand   `xml:"Stand" json:"Area,omitempty"`
}
type StandSlots struct {
	StandSlot []StandSlot `xml:"StandSlot" json:"StandSlot,omitempty"`
}
type Carousel struct {
	Value []Value `xml:"Value" json:"Slot,omitempty"`
	Area  Area    `xml:"Area" json:"Area,omitempty"`
}
type CarouselSlot struct {
	Value    []Value  `xml:"Value" json:"Slot,omitempty"`
	Carousel Carousel `xml:"Carousel" json:"Carousel,omitempty"`
}
type CarouselSlots struct {
	CarouselSlot []CarouselSlot `xml:"CarouselSlot" json:"CarouselSlot,omitempty"`
}

type Gate struct {
	Value []Value `xml:"Value"`
	Area  Area    `xml:"Area"`
}

type GateSlot struct {
	Value []Value `xml:"Value"`
	Gate  Gate    `xml:"Gate"`
}
type GateSlots struct {
	GateSlot []GateSlot `xml:"GateSlot" json:"GateSlot,omitempty"`
}
type CheckIn struct {
	Value []Value `xml:"Value"`
	Area  Area    `xml:"Area"`
}
type CheckInSlot struct {
	Value   []Value `xml:"Value"`
	CheckIn CheckIn `xml:"CheckIn"`
}
type CheckInSlots struct {
	CheckInSlot []CheckInSlot `xml:"CheckInSlot" json:"CheckInSlot,omitempty"`
}
type Chute struct {
	Value []Value `xml:"Value"`
	Area  Area    `xml:"Area"`
}
type ChuteSlot struct {
	Value []Value `xml:"Values"`
	Chute Chute   `xml:"Chute"`
}
type ChuteSlots struct {
	ChuteSlot []ChuteSlot `xml:"ChuteSlot" json:"ChuteSlot,omitempty"`
}

func (fs Flights) DuplicateFlights() Flights {

	x, _ := xml.Marshal(fs)
	var flights Flights
	xml.Unmarshal(x, &flights)
	return flights
}

func (fs Flight) DuplicateFlight() Flight {

	x, err := xml.Marshal(fs)

	if err != nil {
		log.Fatal("Error Marshalling ", err)
	}

	var flight Flight
	xml.Unmarshal(x, &flight)
	return flight
}

type Envelope struct {
	Body struct {
		GetFlightsResponse struct {
			GetFlightsResult struct {
				WebServiceResult struct {
					ApiResponse struct {
						Data struct {
							Flights Flights `xml:"Flights"`
						} `xml:"Data"`
					} `xml:"ApiResponse"`
				} `xml:"WebServiceResult"`
			} `xml:"GetFlightsResult"`
		} `xml:"GetFlightsResponse"`
	} `xml:"Body"`
}

type FlightCreatedNotificationEnvelope struct {
	Content struct {
		FlightCreatedNotification struct {
			Flight Flight `xml:"Flight"`
		} `xml:"FlightCreatedNotification"`
	} `xml:"Content"`
}
type FlightUpdatedNotificationEnvelope struct {
	Content struct {
		FlightUpdatedNotification struct {
			Flight Flight `xml:"Flight"`
		} `xml:"FlightUpdatedNotification"`
	} `xml:"Content"`
}
type FlightDeletedNotificationEnvelope struct {
	Content struct {
		FlightDeletedNotification struct {
			Flight Flight `xml:"Flight"`
		} `xml:"FlightDeletedNotification"`
	} `xml:"Content"`
}

func (f Flight) GetSDO() time.Time {

	sdo := f.FlightId.ScheduledDate
	sdod, _ := time.Parse("2006-01-02", sdo)
	return sdod
}
func (f Flight) GetProperty(property string) string {
	for _, v := range f.FlightState.Value {
		if v.PropertyName == property {
			return v.Text
		}
	}
	return ""
}
func (f Flight) IsArrival() bool {
	if f.FlightId.FlightKind == "Arrival" {
		return true
	} else {
		return false
	}
}
func (f Flight) GetIATAAirline() string {
	for _, v := range f.FlightId.AirlineDesignator {
		if v.CodeContext == "IATA" {
			return v.Text
		}
	}
	return ""
}
func (f Flight) GetIATAAirport() string {
	for _, v := range f.FlightId.AirportCode {
		if v.CodeContext == "IATA" {
			return v.Text
		}
	}
	return ""
}
func (f Flight) GetICAOAirline() string {
	for _, v := range f.FlightId.AirlineDesignator {
		if v.CodeContext == "ICAO" {
			return v.Text
		}
	}
	return ""
}
func (f Flight) GetFlightID() string {

	airline := f.GetIATAAirline()
	fltNum := f.FlightId.FlightNumber
	sto := f.FlightState.ScheduledTime
	// kind := "D"
	// if f.IsArrival() {
	// 	kind = "A"
	// }
	return airline + fltNum + "@" + sto
}
func (f Flight) GetFlightDirection() string {

	if f.IsArrival() {
		return "Arrival"
	} else {
		return "Departure"
	}
}
func (f Flight) GetFlightRoute() string {

	var sb strings.Builder
	idx := 0

	for _, rp := range f.FlightState.Route.ViaPoints.RouteViaPoint {
		for _, ap := range rp.AirportCode {
			if idx > 0 {
				sb.WriteString(",")
			}

			if ap.CodeContext == "IATA" {
				sb.WriteString(ap.Text)
				idx++
			}

		}
	}

	return sb.String()
}
func (f Flight) GetAircraftType() string {

	sb := "-"

	for _, rp := range f.FlightState.AircraftType.AircraftTypeId.AircraftTypeCode {

		if rp.CodeContext == "IATA" {
			sb = rp.Text
		}
	}

	return sb
}
func (f Flight) GetAircraftRegistration() string {

	if f.FlightState.Aircraft.AircraftId.Registration != "" {
		return f.FlightState.Aircraft.AircraftId.Registration
	} else {
		return "-"
	}
}
func (f Flight) GetSTO() time.Time {

	sto := f.FlightState.ScheduledTime

	if sto != "" {
		stot, err := time.ParseInLocation("2006-01-02T15:04:05", sto, timeservice.Loc)
		if err == nil {
			return stot
		}
		return time.Now()
	}

	return time.Now()
}

func (p CheckInSlot) GetResourceID() (name string, from time.Time, to time.Time) {

	for _, v := range p.Value {

		if v.PropertyName == "StartTime" {
			from, _ = time.ParseInLocation(timeservice.Layout, v.Text, timeservice.Loc)
			continue
		}
		if v.PropertyName == "EndTime" {
			to, _ = time.ParseInLocation(timeservice.Layout, v.Text, timeservice.Loc)
			continue
		}
	}

	for _, v := range p.CheckIn.Value {
		if v.PropertyName == "Name" {
			name = v.Text
			continue
		}
	}
	return
}

func (p StandSlot) GetResourceID() (name string, from time.Time, to time.Time) {

	for _, v := range p.Value {

		if v.PropertyName == "StartTime" {
			from, _ = time.ParseInLocation(timeservice.Layout, v.Text, timeservice.Loc)
			continue
		}
		if v.PropertyName == "EndTime" {
			to, _ = time.ParseInLocation(timeservice.Layout, v.Text, timeservice.Loc)
			continue
		}
	}

	for _, v := range p.Stand.Value {
		if v.PropertyName == "Name" {
			name = v.Text
			continue
		}
	}
	return
}
func (p CarouselSlot) GetResourceID() (name string, from time.Time, to time.Time) {

	for _, v := range p.Value {

		if v.PropertyName == "StartTime" {
			from, _ = time.ParseInLocation(timeservice.Layout, v.Text, timeservice.Loc)
			continue
		}
		if v.PropertyName == "EndTime" {
			to, _ = time.ParseInLocation(timeservice.Layout, v.Text, timeservice.Loc)
			continue
		}
	}

	for _, v := range p.Carousel.Value {
		if v.PropertyName == "Name" {
			name = v.Text
			continue
		}
	}
	return
}

func (p ChuteSlot) GetResourceID() (name string, from time.Time, to time.Time) {

	for _, v := range p.Value {

		if v.PropertyName == "StartTime" {
			from, _ = time.ParseInLocation(timeservice.Layout, v.Text, timeservice.Loc)
			continue
		}
		if v.PropertyName == "EndTime" {
			to, _ = time.ParseInLocation(timeservice.Layout, v.Text, timeservice.Loc)
			continue
		}
	}

	for _, v := range p.Chute.Value {
		if v.PropertyName == "Name" {
			name = v.Text
			continue
		}
	}
	return
}

func (p GateSlot) GetResourceID() (name string, from time.Time, to time.Time) {

	for _, v := range p.Value {

		if v.PropertyName == "StartTime" {
			from, _ = time.ParseInLocation(timeservice.Layout, v.Text, timeservice.Loc)
			continue
		}
		if v.PropertyName == "EndTime" {
			to, _ = time.ParseInLocation(timeservice.Layout, v.Text, timeservice.Loc)
			continue
		}
	}

	for _, v := range p.Gate.Value {
		if v.PropertyName == "Name" {
			name = v.Text
			continue
		}
	}
	return name, from, to
}
