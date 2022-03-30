package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type gasData struct {
	// When this data was fetched.
	Timestamp time.Time

	// Unique identifier of the warehouse location.
	Id int

	// User friendly name of the warehouse location.
	Name string

	// Warehouse location latitude and longitude.
	Latitude  float64
	Longitude float64

	// Regular, premium, and diesel gas prices. nil = no gas of this type at
	// this location. Yes, I'm storing currency as a float. Bite me.
	RegularPrice *float64
	PremiumPrice *float64
	DieselPrice  *float64
}

var portFlag = flag.Int("port", 8000, "port to listen on")
var latFlag = flag.Float64("latitude", 0.0, "latitude for search")
var longFlag = flag.Float64("longitude", 0.0, "longitude for search")
var dataFlag = flag.String("data", "", "path to data csv file")
var historyFlag = flag.String("history", "", "path to history csv file")

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

func mustParseFloat64OrEmpty(value string) *float64 {
	if value == "" {
		return nil
	}
	ret := mustParseFloat64(value)
	return &ret
}

func floatToString(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func floatToStringOrEmpty(value *float64) string {
	if value == nil {
		return ""
	}
	return floatToString(*value)
}

func readGastrakCsv(path string) ([]gasData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ret := []gasData{}
	reader := csv.NewReader(f)
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		ret = append(ret, gasData{
			Timestamp:    time.Unix(mustParseInt64(line[0]), 0),
			Id:           int(mustParseInt64(line[1])),
			Name:         line[2],
			Latitude:     mustParseFloat64(line[3]),
			Longitude:    mustParseFloat64(line[4]),
			RegularPrice: mustParseFloat64OrEmpty(line[5]),
			PremiumPrice: mustParseFloat64OrEmpty(line[6]),
			DieselPrice:  mustParseFloat64OrEmpty(line[7]),
		})
	}

	return ret, nil
}

func queryParam(r *http.Request, name string) *string {
	values := r.URL.Query()[name]
	if len(values) == 0 {
		return nil
	}
	return &values[0]
}

func filterName(datas []gasData, name string) []gasData {
	ret := []gasData{}
	for _, data := range datas {
		if strings.EqualFold(data.Name, name) {
			ret = append(ret, data)
		}
	}
	return ret
}

func filterGrade(datas []gasData, grade string) []gasData {
	ret := []gasData{}
	for _, data := range datas {
		if getGradePrice(&data, grade) != nil {
			ret = append(ret, data)
		}
	}
	return ret
}

func getGradePrice(data *gasData, grade string) *float64 {
	if strings.EqualFold(grade, "regular") {
		return data.RegularPrice
	} else if strings.EqualFold(grade, "premium") {
		return data.PremiumPrice
	} else if strings.EqualFold(grade, "diesel") {
		return data.DieselPrice
	} else {
		return nil
	}
}

func csvHistory(datas []gasData, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")

	name := queryParam(r, "name")
	if name != nil {
		datas = filterName(datas, *name)
	}

	grade := queryParam(r, "grade")
	if grade != nil {
		datas = filterGrade(datas, *grade)
	}

	lines := [][]string{}
	for _, data := range datas {
		lines = append(lines, []string{
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

	writer := csv.NewWriter(w)
	err := writer.WriteAll(lines)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func jsonHistory(datas []gasData, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := queryParam(r, "name")
	if name != nil {
		datas = filterName(datas, *name)
	}

	grade := queryParam(r, "grade")
	if grade != nil {
		datas = filterGrade(datas, *grade)
	}

	jsonStr, err := json.Marshal(datas)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(jsonStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func highchartsHistory(datas []gasData, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := queryParam(r, "name")
	grade := queryParam(r, "grade")
	if name == nil || grade == nil {
		http.Error(w, "must specify `name` and `grade` parameters", http.StatusBadRequest)
		return
	}

	datas = filterName(datas, *name)
	datas = filterGrade(datas, *grade)

	points := [][2]float64{}
	for _, data := range datas {
		timestampMs := float64(data.Timestamp.Unix() * 1000)
		price := *getGradePrice(&data, *grade)
		points = append(points, [2]float64{timestampMs, price})
	}

	jsonStr, err := json.Marshal(points)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(jsonStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func history(w http.ResponseWriter, r *http.Request) {
	if *historyFlag == "" {
		http.Error(w, "history not available", http.StatusServiceUnavailable)
		return
	}

	datas, err := readGastrakCsv(*historyFlag)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	format := queryParam(r, "format")
	if format == nil || strings.EqualFold(*format, "csv") {
		csvHistory(datas, w, r)
	} else if strings.EqualFold(*format, "json") {
		jsonHistory(datas, w, r)
	} else if strings.EqualFold(*format, "highcharts") {
		highchartsHistory(datas, w, r)
	} else {
		http.Error(w, "unrecognized format", http.StatusBadRequest)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/index.html.tmpl")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ts, err := os.Stat(*dataFlag)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := readGastrakCsv(*dataFlag)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var args = struct {
		Latitude  float64
		Longitude float64
		Data      interface{}
		Time      string
	}{
		Latitude:  *latFlag,
		Longitude: *longFlag,
		Data:      data,
		Time:      ts.ModTime().Format("2006-01-02"),
	}

	err = t.Execute(w, args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	flag.Parse()
	if *dataFlag == "" || *latFlag == 0.0 || *longFlag == 0.0 {
		fmt.Fprintf(os.Stderr, "usage: server -data=... -latitude=... -longitude=...\n")
		os.Exit(1)
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/history", history)
	http.HandleFunc("/", index)
	http.ListenAndServe(fmt.Sprintf(":%d", *portFlag), nil)
}
