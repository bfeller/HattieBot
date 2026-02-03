package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type GeocodeResult struct {
	Results []struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Country   string  `json:"country"`
	} `json:"results"`
}

type WeatherResponse struct {
	CurrentWeather struct {
		Temperature   float64 `json:"temperature"`
		Windspeed     float64 `json:"windspeed"`
		Winddirection float64 `json:"winddirection"`
		IsDay         int     `json:"is_day"`
		Time          string  `json:"time"`
	} `json:"current_weather"`
}

type Input struct {
	Location string `json:"location"`
}

type Output struct {
	Error       string  `json:"error,omitempty"`
	Location    string  `json:"location"`
	Country     string  `json:"country"`
	Temperature float64 `json:"temperature"`
	Windspeed   float64 `json:"windspeed"`
	WindDir     float64 `json:"wind_direction"`
	IsDay       bool    `json:"is_day"`
	Time        string  `json:"time"`
}

func main() {
	// Read config from stdin
	fmt.Fprintln(os.Stderr, "DEBUG: Starting weather tool...")
	
	var input Input
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG: Failed to parse input: %v\n", err)
		output := Output{Error: fmt.Sprintf("Failed to parse input: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Looking up location: %s\n", input.Location)

	if input.Location == "" {
		output := Output{Error: "No location provided"}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}

	// Geocoding
	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1", 
		strings.ReplaceAll(input.Location, " ", "%20"))
	
	fmt.Fprintf(os.Stderr, "DEBUG: Fetching geocode from: %s\n", geoURL)
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(geoURL)
	if err != nil {
		output := Output{Error: fmt.Sprintf("Geocoding request failed: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var geoResult GeocodeResult
	if err := json.NewDecoder(resp.Body).Decode(&geoResult); err != nil {
		output := Output{Error: fmt.Sprintf("Failed to parse geocoding response: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}

	if len(geoResult.Results) == 0 {
		output := Output{Error: fmt.Sprintf("Location not found: %s", input.Location)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}

	location := geoResult.Results[0]
	fmt.Fprintf(os.Stderr, "DEBUG: Found location: %s, %s (%.4f, %.4f)\n", 
		location.Name, location.Country, location.Latitude, location.Longitude)

	// Weather API
	weatherURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current_weather=true",
		location.Latitude, location.Longitude)
	
	fmt.Fprintf(os.Stderr, "DEBUG: Fetching weather from: %s\n", weatherURL)
	
	resp, err = client.Get(weatherURL)
	if err != nil {
		output := Output{Error: fmt.Sprintf("Weather request failed: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var weather WeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		output := Output{Error: fmt.Sprintf("Failed to parse weather response: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}

	output := Output{
		Location:    location.Name,
		Country:     location.Country,
		Temperature: weather.CurrentWeather.Temperature,
		Windspeed:   weather.CurrentWeather.Windspeed,
		WindDir:     weather.CurrentWeather.Winddirection,
		IsDay:       weather.CurrentWeather.IsDay == 1,
		Time:        weather.CurrentWeather.Time,
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Success! Returning result\n")
	json.NewEncoder(os.Stdout).Encode(output)
}
