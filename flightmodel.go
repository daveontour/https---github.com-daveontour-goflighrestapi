package main

import (
	"encoding/json"
	"encoding/xml"
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
	XMLName           xml.Name            `xml:"FlightId" json:"-"`
	FlightKind        string              `xml:"FlightKind"`
	AirlineDesignator []AirlineDesignator `xml:"AirlineDesignator"`
	FlightNumber      string              `xml:"FlightNumber"`
	ScheduledDate     string              `xml:"ScheduledDate"`
	AirportCode       []AirportCode       `xml:"AirportCode"`
}

func CleanJSON(sb strings.Builder) string {

	s := sb.String()
	if last := len(s) - 1; last >= 0 && s[last] == ',' {
		s = s[:last]
	}

	s = s + "}"

	return s
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
	//Text           string         `xml:",chardata" json:"-"`
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
	//	Text        string    `xml:",chardata" json:"-"`
	CustomsType string    `xml:"customsType,attr"`
	ViaPoints   ViaPoints `xml:"ViaPoints"`
}

type TableValue struct {
	//Text         string  `xml:",chardata" json:"-"`
	PropertyName string  `xml:"propertyName,attr"`
	Value        []Value `xml:"Value"`
}

type StandSlot struct {
	Value []Value `xml:"Value" json:"Slot,omitempty"`
}
type StandSlots struct {
	StandSlot []StandSlot `xml:"StandSlot" json:"StandSlot,omitempty"`
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
		sb.WriteString("}")

	}

	sb.WriteString("]")

	return []byte(sb.String()), nil

}

type CarouselSlot struct {
	Value []Value `xml:"Value" json:"Slot,omitempty"`
}
type CarouselSlots struct {
	CarouselSlot []CarouselSlot `xml:"CarouselSlot" json:"CarouselSlot,omitempty"`
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
		sb.WriteString("}")

	}

	sb.WriteString("]")

	return []byte(sb.String()), nil

}

type GatelSlot struct {
	Value []Value `xml:"Value"`
}
type GateSlots struct {
	GateSlot []GatelSlot `xml:"GateSlot" json:"GateSlot,omitempty"`
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
		sb.WriteString("}")

	}

	sb.WriteString("]")

	return []byte(sb.String()), nil

}

type CheckInSlot struct {
	Value []Value `xml:"Value"`
}
type CheckInSlots struct {
	CheckInSlot []CheckInSlot `xml:"CheckInSlot" json:"CheckInSlot,omitempty"`
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
		sb.WriteString("}")

	}

	sb.WriteString("]")

	return []byte(sb.String()), nil

}

type ChuteSlot struct {
	Value []Value `xml:"Values"`
}
type ChuteSlots struct {
	ChuteSlot []ChuteSlot `xml:"ChuteSlot" json:"ChuteSlot,omitempty"`
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
		sb.WriteString("}")

	}

	sb.WriteString("]")

	return []byte(sb.String()), nil

}

type FlightState struct {
	XMLName       xml.Name      `xml:"FlightState" json:"-"`
	ScheduledTime string        `xml:"ScheduledTime" `
	LinkedFlight  LinkedFlight  `xml:"LinkedFlight"`
	AircraftType  AircraftType  `xml:"AircraftType"`
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

type FlightChanges struct {
	AircraftTypeChange interface{} `xml:"AircraftTypeChange" json:"AircraftTypeChange"`
	// CarouselSlotsChange interface {} `xml:CarouselSlotsChange" json:"CarouselSlotsChange"`
	// GatelSlotsChange interface {} `xml:GateSlotsChange" json:"GateSlotsChange"`
	Changes []Change `xml:"Change"`
}

type Flight struct {
	XMLName       xml.Name      `xml:"Flight" json:"-"`
	FlightId      FlightId      `xml:"FlightId" json:"FlightId"`
	FlightState   FlightState   `xml:"FlightState" json:"FlightState"`
	FlightChanges FlightChanges `xml:"FlightChanges" json:"FlightChanges"`
	//Properties    map[string]string `xml:"-" json:"-"`
}
type Flights struct {
	XMLName xml.Name `xml:"Flights" json:"-"`
	Flight  []Flight `xml:"Flight" json:"Flights"`
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
	XMLName xml.Name `xml:"Envelope"`
	//	Text    string   `xml:",chardata"`
	//	S       string   `xml:"s,attr"`
	Body struct {
		Text               string `xml:",chardata"`
		GetFlightsResponse struct {
			//Text             string `xml:",chardata"`
			Xmlns            string `xml:"xmlns,attr"`
			GetFlightsResult struct {
				Text             string `xml:",chardata"`
				WebServiceResult struct {
					//Text        string `xml:",chardata"`
					ApiVersion  string `xml:"apiVersion,attr"`
					Xsd         string `xml:"xsd,attr"`
					Xsi         string `xml:"xsi,attr"`
					ApiResponse struct {
						Text   string `xml:",chardata"`
						Status struct {
							Text  string `xml:",chardata"`
							Xmlns string `xml:"xmlns,attr"`
						} `xml:"Status"`
						Data struct {
							///	Text    string  `xml:",chardata"`
							Xmlns   string  `xml:"xmlns,attr"`
							Flights Flights `xml:"Flights"`
						} `xml:"Data"`
					} `xml:"ApiResponse"`
				} `xml:"WebServiceResult"`
			} `xml:"GetFlightsResult"`
		} `xml:"GetFlightsResponse"`
	} `xml:"Body"`
}

type FlightCreatedNotificatioEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Content struct {
		FlightCreatedNotification struct {
			Flight Flight `xml:"Flight"`
		} `xml:"FlightCreatedNotification"`
	} `xml:"Content"`
}
type FlightUpdatedNotificatioEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Content struct {
		FlightUpdatedNotification struct {
			Flight Flight `xml:"Flight"`
		} `xml:"FlightUpdatedNotification"`
	} `xml:"Content"`
}
type FlightDeletedNotificatioEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
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

// func (f Flight) InitPropertyMap() {

// 	f.Properties = make(map[string]string)

// 	for _, p := range f.FlightState.Value {
// 		f.Properties[p.PropertyName] = p.Text
// 	}
// }

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
	kind := "D"
	if f.IsArrival() {
		kind = "A"
	}
	return airline + fltNum + kind + "@" + sto
}

func (f Flight) GetSTO() time.Time {

	sto := f.FlightState.ScheduledTime

	if sto != "" {
		stot, err := time.Parse("2006-01-02T15:04:05", sto)
		if err == nil {
			return stot
		}
		return time.Now()
	}

	return time.Now()
}

func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
