package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
)

var portFlag = flag.Int("port", 8000, "port to listen on")
var latFlag = flag.Float64("latitude", 0.0, "latitude for search")
var longFlag = flag.Float64("longitude", 0.0, "longitude for search")
var dataFlag = flag.String("data", "", "path to data csv file")

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

	data, err := ioutil.ReadFile(*dataFlag)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var args = struct {
		Latitude  float64
		Longitude float64
		Data      string
		Time      string
	}{
		Latitude:  *latFlag,
		Longitude: *longFlag,
		Data:      string(data),
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
	http.HandleFunc("/", index)
	http.ListenAndServe(fmt.Sprintf(":%d", *portFlag), nil)
}
