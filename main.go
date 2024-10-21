package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

var defaultRedisPort = 6379

func main() {

	// TODO: maybe add support for cli
	// if location given, maybe don't start server and just call the api

	// args := os.Args[1:]

	// if args[0] == "-h" || args[0] == "--help" {
	// 	fmt.Println("Usage: weather_api <port number> <location> ")
	// 	os.Exit(0)
	// }

	// if len(args) == 0 {
	// 	// no arguments, use default values
	// 	args = []string{"8080", ""}
	// } else {
	// 	// check if the first argument is a port number
	// 	if _, err := strconv.Atoi(args[0]); err != nil {
	// 		fmt.Println("Invalid port number")
	// 		os.Exit(1)
	// 	}

	// 	// check if the second argument is a location
	// 	if len(args[1] == 0) {
	// 		fmt.Println("Invalid location")
	// 		os.Exit(1)
	// 	}
	// }

	rdb, err := getRedisConnection()
	if err != nil {
		log.Println("Cannot connect to Redis:", err)
		os.Exit(1)
	}

	// Get the API key from the environment variable
	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		log.Println("Please set the WEATHER_API_KEY environment variable")
		os.Exit(1)
	}

	// url := log.Sprintf("https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/%s/%s/%s?key=%s", location, date1, date2, apiKey)
	// log.Println(url)

	// Server
	server := &http.Server{
		Addr: ":8080",
	}
	go func() {
		err = startWebServer(server, rdb, apiKey)
		// http.ListenAndServe() returns ErrSeverClosed on error; not nil
		// https://dev.to/mokiat/proper-http-shutdown-in-go-3fji
		if !errors.Is(err, http.ErrServerClosed) {
			log.Println("Cannot start web server:", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM) // respond to SIGINT(ctrl+c) and SIGTERM (system asks the program to terminate gracefully)
	<-sigChan                                               // block until a signal is received

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second) // 10 seconds wait for graceful shutdown
	defer shutdownRelease()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP shutdown error: %v", err)
	}
	log.Println("Graceful shutdown complete.")

}
func startWebServer(server *http.Server, rdb *redis.Client, apiKey string) error {
	if server == nil || rdb == nil || apiKey == "" {
		return fmt.Errorf("invalid arguments provided, cannot start web server")
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rootHandler(w, r, rdb, apiKey)
	})

	log.Printf("Starting server on localhost%s\n", server.Addr)

	return server.ListenAndServe()
}

func getRedisConnection() (*redis.Client, error) {
	redisURL := os.Getenv("REDIS_URL")
	redisPassword := os.Getenv("REDIS_PASSWORD")

	// Check if environment variables are set
	// redis password could be empty
	if redisURL == "" {
		return nil, fmt.Errorf("please set the REDIS_URL and REDIS_PASSWORD environment variables")
	}

	// Create a new Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisURL, defaultRedisPort),
		Password: redisPassword,
		DB:       0, // Use default DB
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return client, nil // Return both the client and nil for no error
}

func rootHandler(w http.ResponseWriter, r *http.Request, rdb *redis.Client, apiKey string) {
	if r.Method != "POST" {
		tmpl, err := template.New("index.html").ParseFiles("index.html") // load the html template
		if err != nil {
			log.Println(err)
			return
		}
		tmpl.Execute(w, nil)
		return
	}

	weatherData, err := getWeatherData(r.FormValue("city"), rdb, apiKey)

	if err != nil {
		http.Error(w, "City not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/json")
	w.Write([]byte(weatherData))
	fmt.Fprintf(w, "City: %s, Weather Data: %s", html.EscapeString(r.FormValue("city")), weatherData)
}

func getWeatherData(city string, rdb *redis.Client, apiKey string) (string, error) {
	city = url.QueryEscape(strings.TrimSpace(html.EscapeString(city)))
	if city == "" {

		return "", fmt.Errorf("city cannot be empty")
	}

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
		return "", fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	weatherData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err // Return an empty string and the error
	} // Return the value and nil for no error

	// Save data in redis
	err = rdb.Set(context.Background(), city, string(weatherData), time.Hour).Err()
	if err != nil {
		return "", err
	}

	return string(weatherData), nil
}
