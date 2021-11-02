package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strconv"
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

func filter(datas []gasData, fn func(gasData) bool) []gasData {
	ret := []gasData{}
	for _, data := range datas {
		if fn(data) {
			ret = append(ret, data)
		}
	}
	return ret
}

func any(values []string, fn func(string) bool) bool {
	for _, value := range values {
		if fn(value) {
			return true
		}
	}
	return false
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

	query := r.URL.Query()
	names, ok := query["name"]
	if ok {
		datas = filter(datas, func(data gasData) bool {
			return any(names, func(name string) bool {
				return name == data.Name
			})
		})
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"history.csv\"")

	lines := [][]string{}
	lines = append(lines, []string{
		"Timestamp",
		"Id",
		"Name",
		"Latitude",
		"Longitude",
		"RegularPrice",
		"PremiumPrice",
		"DieselPrice",
	})

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
	err = writer.WriteAll(lines)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
