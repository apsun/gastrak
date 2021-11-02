package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
)

type gasData struct {
	// When this data was fetched.
	timestamp time.Time

	// Unique identifier of the warehouse location.
	id int

	// User friendly name of the warehouse location.
	locationName string

	// Warehouse location latitude and longitude.
	locationLatitude  float64
	locationLongitude float64

	// Regular, premium, and diesel gas prices. nil = no gas of this type at
	// this location. Yes, I'm storing currency as a float. Bite me.
	regularPrice *float64
	premiumPrice *float64
	dieselPrice  *float64
}

var latFlag = flag.Float64("latitude", 0.0, "latitude for search")
var longFlag = flag.Float64("longitude", 0.0, "longitude for search")

func getGasPrice(warehouseObj map[string]interface{}, key string) *float64 {
	priceMap := warehouseObj["gasPrices"].(map[string]interface{})

	price, ok := priceMap[key]
	if !ok {
		return nil
	}

	priceNum, err := strconv.ParseFloat(price.(string), 64)
	if err != nil {
		panic(err)
	}
	return &priceNum
}

func mustParseWarehouseObj(warehouseObj map[string]interface{}) gasData {
	regularPrice := getGasPrice(warehouseObj, "regular")
	premiumPrice := getGasPrice(warehouseObj, "premium")
	dieselPrice := getGasPrice(warehouseObj, "diesel")

	return gasData{
		timestamp:         time.Now(),
		id:                int(warehouseObj["stlocID"].(float64)),
		locationName:      warehouseObj["locationName"].(string),
		locationLatitude:  warehouseObj["latitude"].(float64),
		locationLongitude: warehouseObj["longitude"].(float64),
		regularPrice:      regularPrice,
		premiumPrice:      premiumPrice,
		dieselPrice:       dieselPrice,
	}
}

func parseWarehouseObj(warehouseObj map[string]interface{}) (ret gasData, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	ret = mustParseWarehouseObj(warehouseObj)
	err = nil
	return
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

func getGasDataNearLocation(latitude, longitude float64) ([]gasData, error) {
	req, err := http.NewRequest(
		"GET",
		"https://www.costco.com/AjaxWarehouseBrowseLookupView",
		nil,
	)
	if err != nil {
		return nil, err
	}

	// Query is limited to 50 results server-side
	q := req.URL.Query()
	q.Add("numOfWarehouses", "50")
	q.Add("hasGas", "true")
	q.Add("populateWarehouseDetails", "true")
	q.Add("latitude", floatToString(latitude))
	q.Add("longitude", floatToString(longitude))
	req.URL.RawQuery = q.Encode()

	// API returns an error unless these headers are set
	req.Header.Add("User-Agent", "Gastrak/1.0")
	req.Header.Add("Accept-Language", "en-US,en;q=0.5")
	req.Header.Add("Accept", "*/*")

	// Perform HTTP request, parse response as a JSON list
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	jsonBody := []interface{}{}
	err = json.Unmarshal(body, &jsonBody)
	if err != nil {
		return nil, err
	}

	// First element is a boolean for some weird reason, so skip [0]
	ret := []gasData{}
	for _, warehouseObj := range jsonBody[1:] {
		data, err := parseWarehouseObj(warehouseObj.(map[string]interface{}))
		if err != nil {
			return nil, err
		}
		ret = append(ret, data)
	}
	return ret, nil
}

func main() {
	flag.Parse()

	datapoints, err := getGasDataNearLocation(*latFlag, *longFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	lines := [][]string{}
	for _, data := range datapoints {
		lines = append(lines, []string{
			strconv.FormatInt(data.timestamp.Unix(), 10),
			strconv.Itoa(data.id),
			data.locationName,
			floatToString(data.locationLatitude),
			floatToString(data.locationLongitude),
			floatToStringOrEmpty(data.regularPrice),
			floatToStringOrEmpty(data.premiumPrice),
			floatToStringOrEmpty(data.dieselPrice),
		})
	}

	writer := csv.NewWriter(os.Stdout)
	err = writer.WriteAll(lines)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
