package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"

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
	// apiKey := os.Getenv("WEATHER_API_KEY")
	// if apiKey == "" {
	// 	fmt.Println("Please set the WEATHER_API_KEY environment variable")
	// 	os.Exit(1)
	// }

	// url := "https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/"

	// fmt.Println(url)

	StartWebServer()
}

func StartWebServer() {
	http.HandleFunc("/", RootHandler)

	fmt.Println("Listening on port localhost:8080")
	http.ListenAndServe(":8080", nil)
}

func GetRedisConnection() *redis.Client {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		fmt.Println("Please set the REDIS_URL environment variable")
		return nil
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword == "" {
		fmt.Println("Please set the REDIS_PASSWORD environment variable")
		return nil
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	return rdb
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			fmt.Println("Error parsing form:", err)
			return
		}
		// Handle form submission
		city := r.FormValue("city")
		fmt.Fprintf(w, "City: %s", city)
		return
	}

	tmpl, err := template.ParseFiles("index.html") // load the html template
	if err != nil {
		fmt.Println(err)
	}
	tmpl.Execute(w, nil)
}
