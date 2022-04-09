package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type stationData struct {
	// Unique identifier of the warehouse location.
	Id int

	// User friendly name of the warehouse location.
	Name string

	// Warehouse location latitude and longitude.
	Latitude  float64
	Longitude float64
}

type gasData struct {
	*stationData

	// When this data was fetched.
	Timestamp time.Time

	// Regular, premium, and diesel gas prices. 0 = no gas of this type at
	// this location. Yes, I'm storing currency as a float. Bite me.
	RegularPrice float64
	PremiumPrice float64
	DieselPrice  float64
}

var portFlag = flag.Int("port", 8000, "port to listen on")
var latFlag = flag.Float64("latitude", 0, "latitude for search")
var lngFlag = flag.Float64("longitude", 0, "longitude for search")
var dataFlag = flag.String("data", "", "path to data csv file")
var historyFlag = flag.String("history", "", "path to history csv file")
var refreshFlag = flag.Duration("refresh", 60*time.Second, "how often to refresh data")

func mustParseInt64(value string) int64 {
	ret, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		panic(err)
	}
	return ret
}

func mustParseFloat64(value string) float64 {
	ret, err := strconv.ParseFloat(value, 64)
	if err != nil {
		panic(err)
	}
	return ret
}

func mustParseFloat64OrEmpty(value string) float64 {
	if value == "" {
		return 0
	}
	return mustParseFloat64(value)
}

func floatToString(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func floatToStringOrEmpty(value float64) string {
	if value == 0 {
		return ""
	}
	return floatToString(value)
}

func readDataCSV(path string) ([]gasData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open data file: %w", err)
	}
	defer f.Close()

	stations := map[int]*stationData{}
	ret := []gasData{}
	reader := csv.NewReader(f)
	reader.FieldsPerRecord = 8
	reader.ReuseRecord = true
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to read data file: %w", err)
		}

		stationId := int(mustParseInt64(line[1]))
		station := stations[stationId]
		if station == nil {
			stationName := line[2]
			stationLat := mustParseFloat64(line[3])
			stationLng := mustParseFloat64(line[4])
			station = &stationData{
				Name:      stationName,
				Id:        stationId,
				Latitude:  stationLat,
				Longitude: stationLng,
			}
			stations[stationId] = station
		}

		ret = append(ret, gasData{
			stationData:  station,
			Timestamp:    time.Unix(mustParseInt64(line[0]), 0),
			RegularPrice: mustParseFloat64OrEmpty(line[5]),
			PremiumPrice: mustParseFloat64OrEmpty(line[6]),
			DieselPrice:  mustParseFloat64OrEmpty(line[7]),
		})
	}

	return ret, nil
}

var data = struct {
	mu        sync.RWMutex
	updatedAt time.Time
	current   []gasData
	history   []gasData
}{}

func refreshOnce() error {
	stat, err := os.Stat(*dataFlag)
	if err != nil {
		return fmt.Errorf("failed to stat current data: %w", err)
	}
	updatedAt := stat.ModTime()

	current, err := readDataCSV(*dataFlag)
	if err != nil {
		return fmt.Errorf("failed to read current data: %w", err)
	}

	var history []gasData
	if *historyFlag != "" {
		history, err = readDataCSV(*historyFlag)
		if err != nil {
			return fmt.Errorf("failed to read history data: %w", err)
		}
	}

	func() {
		data.mu.Lock()
		defer data.mu.Unlock()
		data.updatedAt = updatedAt
		data.current = current
		data.history = history
	}()

	return nil
}

func refreshPeriodic() {
	for {
		time.Sleep(*refreshFlag)
		err := refreshOnce()
		if err != nil {
			log.Printf("failed to refresh data: %v\n", err)
		}
	}
}

func getGradePrice(data *gasData, grade string) float64 {
	if strings.EqualFold(grade, "regular") {
		return data.RegularPrice
	} else if strings.EqualFold(grade, "premium") {
		return data.PremiumPrice
	} else if strings.EqualFold(grade, "diesel") {
		return data.DieselPrice
	} else {
		return 0
	}
}

func queryParam(r *http.Request, name string) string {
	values := r.URL.Query()[name]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

type filter struct {
	name  string
	grade string
}

func newFilterFromQuery(r *http.Request) filter {
	return filter{
		name:  queryParam(r, "name"),
		grade: queryParam(r, "grade"),
	}
}

func (f *filter) match(data *gasData) bool {
	if f.name != "" && !strings.EqualFold(data.Name, f.name) {
		return false
	}

	if f.grade != "" && getGradePrice(data, f.grade) == 0 {
		return false
	}

	return true
}

func internalHTTPError(w http.ResponseWriter, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	log.Println(msg)
	http.Error(w, msg, http.StatusInternalServerError)
}

func serveCSV(datas []gasData, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	filter := newFilterFromQuery(r)

	writer := csv.NewWriter(w)
	for i := range datas {
		data := &datas[i]
		if !filter.match(data) {
			continue
		}

		writer.Write([]string{
			strconv.FormatInt(data.Timestamp.Unix(), 10),
			strconv.Itoa(data.Id),
			data.Name,
			floatToString(data.Latitude),
			floatToString(data.Longitude),
			floatToStringOrEmpty(data.RegularPrice),
			floatToStringOrEmpty(data.PremiumPrice),
			floatToStringOrEmpty(data.DieselPrice),
		})
	}

	writer.Flush()
	err := writer.Error()
	if err != nil {
		internalHTTPError(w, "failed to write response: %v", err)
		return
	}
}

func serveJSON(datas []gasData, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	filter := newFilterFromQuery(r)

	filtered := []gasData{}
	for i := range datas {
		data := &datas[i]
		if !filter.match(data) {
			continue
		}

		filtered = append(filtered, *data)
	}

	jsonStr, err := json.Marshal(filtered)
	if err != nil {
		internalHTTPError(w, "failed to marshal json: %v", err)
		return
	}

	_, err = w.Write(jsonStr)
	if err != nil {
		internalHTTPError(w, "failed to write response: %v", err)
		return
	}
}

func serveHighcharts(datas []gasData, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	filter := newFilterFromQuery(r)
	if filter.name == "" || filter.grade == "" {
		http.Error(w, "must specify `name` and `grade` parameters", http.StatusBadRequest)
		return
	}

	points := [][2]float64{}
	for i := range datas {
		data := &datas[i]
		if !filter.match(data) {
			continue
		}

		timestampMs := float64(data.Timestamp.Unix() * 1000)
		price := getGradePrice(data, filter.grade)
		points = append(points, [2]float64{timestampMs, price})
	}

	jsonStr, err := json.Marshal(points)
	if err != nil {
		internalHTTPError(w, "failed to marshal json: %v", err)
		return
	}

	_, err = w.Write(jsonStr)
	if err != nil {
		internalHTTPError(w, "failed to write response: %v", err)
		return
	}
}

func serveFormat(datas []gasData, w http.ResponseWriter, r *http.Request) {
	format := queryParam(r, "format")
	if format == "" || strings.EqualFold(format, "csv") {
		serveCSV(datas, w, r)
	} else if strings.EqualFold(format, "json") {
		serveJSON(datas, w, r)
	} else if strings.EqualFold(format, "highcharts") {
		serveHighcharts(datas, w, r)
	} else {
		http.Error(w, "unrecognized format", http.StatusBadRequest)
		return
	}
}

func history(w http.ResponseWriter, r *http.Request) {
	if *historyFlag == "" {
		http.Error(w, "history not available", http.StatusServiceUnavailable)
		return
	}

	history := func() []gasData {
		data.mu.RLock()
		defer data.mu.RUnlock()
		return data.history
	}()

	serveFormat(history, w, r)
}

func current(w http.ResponseWriter, r *http.Request) {
	current := func() []gasData {
		data.mu.RLock()
		defer data.mu.RUnlock()
		return data.current
	}()

	serveFormat(current, w, r)
}

func index(w http.ResponseWriter, r *http.Request) {
	updatedAt, current := func() (time.Time, []gasData) {
		data.mu.RLock()
		defer data.mu.RUnlock()
		return data.updatedAt, data.current
	}()

	t, err := template.ParseFiles("templates/index.html.tmpl")
	if err != nil {
		internalHTTPError(w, "failed to parse template: %v", err)
		return
	}

	var args = struct {
		Latitude  float64
		Longitude float64
		Data      interface{}
		Time      int64
	}{
		Latitude:  *latFlag,
		Longitude: *lngFlag,
		Data:      current,
		Time:      updatedAt.Unix() * 1000,
	}

	err = t.Execute(w, args)
	if err != nil {
		internalHTTPError(w, "failed to render template: %v", err)
		return
	}
}

func main() {
	flag.Parse()
	if *dataFlag == "" || *latFlag == 0 || *lngFlag == 0 {
		fmt.Fprintf(os.Stderr, "usage: server -data=... -latitude=... -longitude=...\n")
		os.Exit(1)
	}

	err := refreshOnce()
	if err != nil {
		log.Fatalf("failed to initialize data: %v\n", err)
	}
	go refreshPeriodic()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/history", history)
	http.HandleFunc("/current", current)
	http.HandleFunc("/", index)
	http.ListenAndServe(fmt.Sprintf(":%d", *portFlag), nil)
}
