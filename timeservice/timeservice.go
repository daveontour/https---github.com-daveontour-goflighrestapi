package timeservice

import "time"

const Layout = "2006-01-02T15:04:05"

var Loc *time.Location

func InitTimeService() {
	Loc, _ = time.LoadLocation("Local")
}
