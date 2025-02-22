package main

import (
	"database/sql"
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
	"time"

	_ "github.com/mattn/go-sqlite3"
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
	RegularPrice float64 `json:",omitempty"`
	PremiumPrice float64 `json:",omitempty"`
	DieselPrice  float64 `json:",omitempty"`
}

var portFlag = flag.Int("port", 8000, "port to listen on")
var latFlag = flag.Float64("latitude", 0, "latitude for search")
var lngFlag = flag.Float64("longitude", 0, "longitude for search")
var currentFlag = flag.String("current", "", "path to current data csv file")
var historyFlag = flag.String("history", "", "path to history sqlite db file")

var historyDB *sql.DB

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

func readDataSQL(db *sql.DB, query string, args ...interface{}) ([]gasData, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query db: %w", err)
	}
	defer rows.Close()

	stations := map[int]*stationData{}
	ret := []gasData{}
	for rows.Next() {
		var ts int64
		var stationId int
		var stationName string
		var stationLat float64
		var stationLng float64
		var regularPrice string
		var premiumPrice string
		var dieselPrice string
		err := rows.Scan(
			&ts,
			&stationId,
			&stationName,
			&stationLat,
			&stationLng,
			&regularPrice,
			&premiumPrice,
			&dieselPrice,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		station := stations[stationId]
		if station == nil {
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
			Timestamp:    time.Unix(ts, 0),
			RegularPrice: mustParseFloat64OrEmpty(regularPrice),
			PremiumPrice: mustParseFloat64OrEmpty(premiumPrice),
			DieselPrice:  mustParseFloat64OrEmpty(dieselPrice),
		})
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare row: %w", err)
	}

	return ret, nil
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

type query struct {
	name  string
	grade string
}

func getHTTPQueryParam(r *http.Request, name string) string {
	values := r.URL.Query()[name]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func queryFromHTTPRequest(r *http.Request) query {
	return query{
		name:  getHTTPQueryParam(r, "name"),
		grade: getHTTPQueryParam(r, "grade"),
	}
}

func queryToSQL(q query) (string, []interface{}) {
	qs := "SELECT * FROM data WHERE 1=1"
	args := []interface{}{}

	if q.name != "" {
		qs += " AND name = ?"
		args = append(args, q.name)
	}

	if q.grade != "" {
		if strings.EqualFold(q.grade, "regular") {
			qs += " AND regular_price != ''"
		} else if strings.EqualFold(q.grade, "premium") {
			qs += " AND premium_price != ''"
		} else if strings.EqualFold(q.grade, "diesel") {
			qs += " AND diesel_price != ''"
		}
	}

	qs += " ORDER BY time"
	return qs, args
}

func internalHTTPError(w http.ResponseWriter, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	log.Println(msg)
	http.Error(w, msg, http.StatusInternalServerError)
}

func serveCSV(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	q := queryFromHTTPRequest(r)

	sq, args := queryToSQL(q)
	datas, err := readDataSQL(db, sq, args...)
	if err != nil {
		internalHTTPError(w, "failed to query data: %v", err)
		return
	}

	writer := csv.NewWriter(w)
	for i := range datas {
		data := &datas[i]
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
	err = writer.Error()
	if err != nil {
		internalHTTPError(w, "failed to write response: %v", err)
		return
	}
}

func serveJSON(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	q := queryFromHTTPRequest(r)

	sq, args := queryToSQL(q)
	datas, err := readDataSQL(db, sq, args...)
	if err != nil {
		internalHTTPError(w, "failed to query data: %v", err)
		return
	}

	jsonStr, err := json.Marshal(datas)
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

func serveTimeseries(db *sql.DB, w http.ResponseWriter, r *http.Request, transpose bool) {
	w.Header().Set("Content-Type", "application/json")
	q := queryFromHTTPRequest(r)
	if q.name == "" || q.grade == "" {
		http.Error(w, "must specify `name` and `grade` parameters", http.StatusBadRequest)
		return
	}

	sq, args := queryToSQL(q)
	datas, err := readDataSQL(db, sq, args...)
	if err != nil {
		internalHTTPError(w, "failed to query data: %v", err)
		return
	}

	var pointsIf interface{}
	if transpose {
		points := [2][]float64{}
		for i := range datas {
			data := &datas[i]
			timestamp := float64(data.Timestamp.Unix())
			price := getGradePrice(data, q.grade)
			points[0] = append(points[0], timestamp)
			points[1] = append(points[1], price)
		}
		pointsIf = points
	} else {
		points := [][2]float64{}
		for i := range datas {
			data := &datas[i]
			timestamp := float64(data.Timestamp.Unix())
			price := getGradePrice(data, q.grade)
			points = append(points, [2]float64{timestamp, price})
		}
		pointsIf = points
	}

	jsonStr, err := json.Marshal(pointsIf)
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

func history(w http.ResponseWriter, r *http.Request) {
	if historyDB == nil {
		http.Error(w, "history not available", http.StatusServiceUnavailable)
		return
	}

	format := getHTTPQueryParam(r, "format")
	if format == "" || strings.EqualFold(format, "csv") {
		serveCSV(historyDB, w, r)
	} else if strings.EqualFold(format, "json") {
		serveJSON(historyDB, w, r)
	} else if strings.EqualFold(format, "timeseries") {
		serveTimeseries(historyDB, w, r, false)
	} else if strings.EqualFold(format, "timeseries-transposed") {
		serveTimeseries(historyDB, w, r, true)
	} else {
		http.Error(w, "unrecognized format", http.StatusBadRequest)
		return
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	stat, err := os.Stat(*currentFlag)
	if err != nil {
		internalHTTPError(w, "failed to stat current data: %v", err)
		return
	}

	data, err := readDataCSV(*currentFlag)
	if err != nil {
		internalHTTPError(w, "failed to read current data: %v", err)
		return
	}

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
		Data:      data,
		Time:      stat.ModTime().Unix() * 1000,
	}

	err = t.Execute(w, args)
	if err != nil {
		internalHTTPError(w, "failed to render template: %v", err)
		return
	}
}

func main() {
	flag.Parse()
	if *currentFlag == "" || *latFlag == 0 || *lngFlag == 0 {
		fmt.Fprintf(os.Stderr, "usage: server -current=... -latitude=... -longitude=...\n")
		os.Exit(1)
	}

	if *historyFlag != "" {
		db, err := sql.Open("sqlite3", *historyFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open history db: %v\n", err)
			os.Exit(1)
		}
		historyDB = db
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/history", history)
	http.HandleFunc("/", index)
	http.ListenAndServe(fmt.Sprintf(":%d", *portFlag), nil)
}
