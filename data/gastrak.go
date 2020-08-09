package main

import (
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

func getGasPrice(warehouseObj map[string]interface{}, key string) (*float64, error) {
	priceMap := warehouseObj["gasPrices"].(map[string]interface{})

	price, ok := priceMap[key]
	if !ok {
		return nil, nil
	}

	priceNum, err := strconv.ParseFloat(price.(string), 64)
	if err != nil {
		return nil, err
	}
	return &priceNum, nil
}

func parseWarehouseObj(warehouseObj map[string]interface{}) (*gasData, error) {
	regularPrice, err := getGasPrice(warehouseObj, "regular")
	if err != nil {
		return nil, err
	}

	premiumPrice, err := getGasPrice(warehouseObj, "premium")
	if err != nil {
		return nil, err
	}

	dieselPrice, err := getGasPrice(warehouseObj, "diesel")
	if err != nil {
		return nil, err
	}

	return &gasData{
		timestamp:         time.Now(),
		id:                int(warehouseObj["stlocID"].(float64)),
		locationName:      warehouseObj["locationName"].(string),
		locationLatitude:  warehouseObj["latitude"].(float64),
		locationLongitude: warehouseObj["longitude"].(float64),
		regularPrice:      regularPrice,
		premiumPrice:      premiumPrice,
		dieselPrice:       dieselPrice,
	}, nil
}

func floatToStringOrEmpty(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func getGasDataNearLocation(latitude, longitude float64) (*[]gasData, error) {
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
	q.Add("latitude", floatToStringOrEmpty(&latitude))
	q.Add("longitude", floatToStringOrEmpty(&longitude))
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
		ret = append(ret, *data)
	}
	return &ret, nil
}

func main() {
	flag.Parse()

	datapoints, err := getGasDataNearLocation(*latFlag, *longFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	for _, data := range *datapoints {
		fmt.Printf("%d,%d,%s,%f,%f,%s,%s,%s\n",
			data.timestamp.Unix(),
			data.id,
			data.locationName,
			data.locationLatitude,
			data.locationLongitude,
			floatToStringOrEmpty(data.regularPrice),
			floatToStringOrEmpty(data.premiumPrice),
			floatToStringOrEmpty(data.dieselPrice),
		)
	}
}
