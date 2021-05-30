package utils

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func GetUtc(url string) time.Time {
	var utctime UtcTime
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Unable to get the time from URL: %v\n", err)
		return time.Time{}
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Failed to close resp.Body: %v", err)
		}
	}(resp.Body)

	data, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(data, &utctime)
	if err != nil {
		log.Printf("Unable to unmarshal utc time api response into UtcTime struct: %v", err)
	}

	utc, err := time.Parse("2006-01-02 15:04:05", utctime.Formatted)
	if err != nil {
		log.Printf("Unable to parse provided UTC string: %v\n", err)
	}
	return utc
}

func SetHoursAsInts(utc time.Time) []int {
	var hoursAsInts []int
	tzset := []string{"America/Panama", "Africa/Algiers", "Asia/Jakarta"}
	for _, tz := range tzset {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			log.Printf("Unable to set location: %v\n", err)
		}
		zonetime := utc.In(loc)
		if err != nil {
			log.Printf("Unable to get time in %v location: %v\n", tz, err)
		}
		hourAsInt, err := strconv.Atoi(strings.Split((strings.Split(zonetime.String(), " ")[1]), ":")[0])
		if err != nil {
			log.Printf("Unable to convert string representation of time zone hour to int: %v\n", err)
		}

		hoursAsInts = append(hoursAsInts, hourAsInt)
	}
	return hoursAsInts
}

// example usage
// utc := GetUtc("redacted")
// hoursAsInts := SetHoursAsInts(utc)
