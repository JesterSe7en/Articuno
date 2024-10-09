package main

import (
	"context"
	"fmt"
	"html"
	"html/template"
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

// GetRedisConnection creates a Redis client object based on the REDIS_URL and
// REDIS_PASSWORD environment variables. If either of these variables is not
// set, it prints an error message and returns nil. Otherwise, it returns a
// Redis client object.
func GetRedisConnection() *redis.Client {
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	redisPassword := strings.TrimSpace(os.Getenv("REDIS_PASSWORD"))
	if redisURL == "" || redisPassword == "" {
		fmt.Println("Please set the REDIS_URL and REDIS_PASSWORD environment variables")
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
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
	fmt.Printf(weatherData)
	fmt.Fprintf(w, "City: %s", city)
}

func getWeatherData(city string, rdb *redis.Client, apiKey string) (string, error) {
	// Check redis cache, if not in redis, fetch from API
	val, err := rdb.Get(context.Background(), city).Result()

	// Check if the key exists in redis - must use redis.nil to compare
	if err != redis.Nil {
		return "", err // Return an empty string and the error
	} else if val != "" {
		return val, nil // Return the value and nil for no error
	}

	// no data in redis, fetch from API
	url := fmt.Sprintf("https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/%s?key=%s", city, apiKey)
	r, err := http.Get(url)
	if err != nil {
		return "", err
	} else if r.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Request failed with status code: %d", r.StatusCode)
	}
	defer r.Body.Close()

	return "", nil
}
