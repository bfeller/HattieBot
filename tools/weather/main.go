package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// Input structure for the tool
type Input struct {
	City string `json:"city"`
}

// Output structure for the tool
type Output struct {
	City        string  `json:"city"`
	Temperature float64 `json:"temperature"`
	Unit        string  `json:"unit"`
	Condition   string  `json:"condition"`
	WindSpeed   float64 `json:"wind_speed"`
	Humidity    int     `json:"humidity"`
	Summary     string  `json:"summary"`
	Error       string  `json:"error,omitempty"`
}

// Geocoding response
type GeoResponse struct {
	Results []struct {
		Name    string  `json:"name"`
		Country string  `json:"country"`
		Lat     float64 `json:"latitude"`
		Lon     float64 `json:"longitude"`
	} `json:"results"`
}

// Weather response
type WeatherResponse struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		Humidity    int     `json:"relative_humidity_2m"`
		WindSpeed   float64 `json:"wind_speed_10m"`
		WeatherCode int     `json:"weather_code"`
	} `json:"current"`
}

func getWeatherCodeDesc(code int) string {
	codes := map[int]string{
		0:  "Clear sky",
		1:  "Mainly clear",
		2:  "Partly cloudy",
		3:  "Overcast",
		45: "Foggy",
		48: "Depositing rime fog",
		51: "Light drizzle",
		53: "Moderate drizzle",
		55: "Dense drizzle",
		61: "Slight rain",
		63: "Moderate rain",
		65: "Heavy rain",
		71: "Slight snow",
		73: "Moderate snow",
		75: "Heavy snow",
		77: "Snow grains",
		80: "Slight rain showers",
		81: "Moderate rain showers",
		82: "Violent rain showers",
		85: "Slight snow showers",
		86: "Heavy snow showers",
		95: "Thunderstorm",
		96: "Thunderstorm with hail",
		99: "Thunderstorm with heavy hail",
	}
	if desc, ok := codes[code]; ok {
		return desc
	}
	return "Unknown"
}

func main() {
	// Read input from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		output := Output{Error: fmt.Sprintf("Failed to read input: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(0)
	}

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		output := Output{Error: fmt.Sprintf("Invalid input JSON: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(0)
	}

	if input.City == "" {
		output := Output{Error: "City name is required"}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(0)
	}

	// Step 1: Get coordinates from geocoding API
	encodedCity := url.QueryEscape(input.City)
	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en&format=json", encodedCity)
	geoResp, err := http.Get(geoURL)
	if err != nil {
		output := Output{Error: fmt.Sprintf("Failed to geocode city: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(0)
	}
	defer geoResp.Body.Close()

	var geoData GeoResponse
	if err := json.NewDecoder(geoResp.Body).Decode(&geoData); err != nil {
		output := Output{Error: fmt.Sprintf("Failed to parse geocoding response: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(0)
	}

	if len(geoData.Results) == 0 {
		output := Output{Error: fmt.Sprintf("City '%s' not found", input.City)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(0)
	}

	loc := geoData.Results[0]

	// Step 2: Get weather data
	weatherURL := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,relative_humidity_2m,wind_speed_10m,weather_code&temperature_unit=celsius&wind_speed_unit=kmh", loc.Lat, loc.Lon)
	weatherResp, err := http.Get(weatherURL)
	if err != nil {
		output := Output{Error: fmt.Sprintf("Failed to fetch weather: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(0)
	}
	defer weatherResp.Body.Close()

	var weatherData WeatherResponse
	if err := json.NewDecoder(weatherResp.Body).Decode(&weatherData); err != nil {
		output := Output{Error: fmt.Sprintf("Failed to parse weather response: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(0)
	}

	current := weatherData.Current

	output := Output{
		City:        loc.Name,
		Temperature: current.Temperature,
		Unit:        "°C",
		Condition:   getWeatherCodeDesc(current.WeatherCode),
		WindSpeed:   current.WindSpeed,
		Humidity:    current.Humidity,
		Summary:     fmt.Sprintf("%s, %s: %.1f°C, %s, Wind: %.1f km/h, Humidity: %d%%", loc.Name, loc.Country, current.Temperature, getWeatherCodeDesc(current.WeatherCode), current.WindSpeed, current.Humidity),
	}

	json.NewEncoder(os.Stdout).Encode(output)
}
