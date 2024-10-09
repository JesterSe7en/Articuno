package main

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
)

func main() {

	rdb := GetRedisConnection()
	if rdb == nil {
		os.Exit(1)
	}

	// Ping the redis server
	err := rdb.Ping(context.Background()).Err()
	if err != nil {
		fmt.Println("Cannot ping redis server:", err)
		os.Exit(1)
	}

	// Get the API key from the environment variable
	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set the WEATHER_API_KEY environment variable")
		os.Exit(1)
	}

	// url := fmt.Sprintf("https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/%s/%s/%s?key=%s", location, date1, date2, apiKey)
	// fmt.Println(url)

	StartWebServer(rdb, apiKey)
}
func StartWebServer(rdb *redis.Client, apiKey string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		RootHandler(w, r, rdb, apiKey)
	})

	fmt.Println("Listening on port localhost:8080")
	http.ListenAndServe(":8080", nil)
}

func GetRedisConnection() *redis.Client {
	redisURL := os.Getenv("REDIS_URL")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisURL == "" || redisPassword == "" {
		fmt.Println("Please set the REDIS_URL and REDIS_PASSWORD environment variables")
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
	})
}
func RootHandler(w http.ResponseWriter, r *http.Request, rdb *redis.Client, apiKey string) {
	if r.Method != "POST" {
		tmpl, err := template.New("index.html").ParseFiles("index.html") // load the html template
		if err != nil {
			fmt.Println(err)
			return
		}
		tmpl.Execute(w, nil)
		return
	}

	city := html.EscapeString(r.FormValue("city"))
	weatherData, err := getWeatherData(city, rdb, apiKey)

	if err != nil {
		http.Error(w, "City not found", http.StatusNotFound)
		return
	}
	fmt.Fprintf(w, "City: %s \nWeather Data: %s ", city, weatherData)
}

func getWeatherData(city string, rdb *redis.Client, apiKey string) (string, error) {
	// Check redis cache, if not in redis, fetch from API
	val, _ := rdb.Get(context.Background(), city).Result()

	if val != "" {
		return val, nil
	}

	// no data in redis, fetch from API; must use query parameters to set api key (visualcrossing.com expects this)
	url := fmt.Sprintf("https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/%s?key=%s", city, apiKey)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	} else if resp.StatusCode != 200 {
		return "", fmt.Errorf("Request failed with status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	weatherData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err // Return an empty string and the error
	} // Return the value and nil for no error

	// Save data in redis
	err = rdb.Set(context.Background(), city, string(weatherData), 0).Err()
	if err != nil {
		return "", err
	}

	return string(weatherData), nil
}
