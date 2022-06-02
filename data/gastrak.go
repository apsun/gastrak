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
	Timestamp time.Time

	// Unique identifier of the warehouse location.
	Id int

	// User friendly name of the warehouse location.
	Name string

	// Warehouse location latitude and longitude.
	Latitude  float64
	Longitude float64

	// Regular, premium, and diesel gas prices. 0 = no gas of this type at
	// this location. Yes, I'm storing currency as a float. Bite me.
	RegularPrice float64
	PremiumPrice float64
	DieselPrice  float64
}

var latFlag = flag.Float64("latitude", 0, "latitude for search")
var lngFlag = flag.Float64("longitude", 0, "longitude for search")

func getGasPrice(warehouseObj map[string]interface{}, key string) float64 {
	priceMap := warehouseObj["gasPrices"].(map[string]interface{})

	price, ok := priceMap[key]
	if !ok {
		return 0
	}

	priceNum, err := strconv.ParseFloat(price.(string), 64)
	if err != nil {
		panic(err)
	}
	return priceNum
}

func mustParseWarehouseObj(warehouseObj map[string]interface{}) gasData {
	regularPrice := getGasPrice(warehouseObj, "regular")
	premiumPrice := getGasPrice(warehouseObj, "premium")
	dieselPrice := getGasPrice(warehouseObj, "diesel")

	return gasData{
		Timestamp:    time.Now(),
		Id:           int(warehouseObj["stlocID"].(float64)),
		Name:         warehouseObj["locationName"].(string),
		Latitude:     warehouseObj["latitude"].(float64),
		Longitude:    warehouseObj["longitude"].(float64),
		RegularPrice: regularPrice,
		PremiumPrice: premiumPrice,
		DieselPrice:  dieselPrice,
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

func floatToStringOrEmpty(value float64) string {
	if value == 0 {
		return ""
	}
	return floatToString(value)
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
	q.Add("countryCode", "US")
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

	datapoints, err := getGasDataNearLocation(*latFlag, *lngFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	lines := [][]string{}
	for _, data := range datapoints {
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

	writer := csv.NewWriter(os.Stdout)
	err = writer.WriteAll(lines)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
