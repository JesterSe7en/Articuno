package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

var testRedisClient *redis.Client
var server = &http.Server{
	Addr: ":8080",
}
var sigChan = make(chan os.Signal, 1)

func setup() error {

	// Initalize the redis client
	if testRedisClient != nil {
		return fmt.Errorf("attempted to setup test redis client twice")
	}

	redisURL := os.Getenv("REDIS_URL")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisURL == "" || redisPassword == "" {
		return fmt.Errorf("please set the REDIS_URL and REDIS_PASSWORD environment variables")
	}

	testRedisClient = redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})

	// Prepare test data
	testingData := [...]struct {
		city        string
		weatherData string
	}{
		{"London", `{"location": "London", "temperature": "15°C"}`},
		{"Paris", `{"location": "Paris", "temperature": "18°C"}`},
		{"Berlin", `{"location": "Berlin", "temperature": "13°C"}`},
		{"Amsterdam", `{"location": "Amsterdam", "temperature": "14°C"}`},
	}

	pipe := testRedisClient.Pipeline()

	for _, data := range testingData {
		pipe.Set(context.Background(), data.city, data.weatherData, 0)
	}
	cmds, err := pipe.Exec(context.Background())

	if err != nil {
		return err
	}

	for _, cmd := range cmds {
		if cmd.Err() != nil {
			return cmd.Err()
		}
	}

	go func() {
		if err := startWebServer(server, testRedisClient, os.Getenv("WEATHER_API_KEY")); err != nil {
			log.Fatalf("Failed to start web server: %v", err)
		}
	}()

	go func() {
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM) // respond to SIGINT(ctrl+c) and SIGTERM (system asks the program to terminate gracefully)
		<-sigChan                                               // block until a signal is received
	}()

	return nil
}

func teardown() {
	if testRedisClient != nil {
		testRedisClient.Close()
		testRedisClient = nil
	}

	close(sigChan)

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second) // 10 seconds wait for graceful shutdown
	defer shutdownRelease()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP shutdown error: %v", err)
	}
	log.Println("Graceful shutdown complete.")
}

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		fmt.Println("Test setup failed:", err)
		teardown()
		os.Exit(1)
	}

	defer teardown()
	m.Run()
}

func TestGetWeatherData(t *testing.T) {

	testingData := []struct {
		city        string
		weatherData string
	}{
		{"London", `{"location": "London", "temperature": "15°C"}`},
		{"Paris", `{"location": "Paris", "temperature": "18°C"}`},
		{"Berlin", `{"location": "Berlin", "temperature": "13°C"}`},
		{"Amsterdam", `{"location": "Amsterdam", "temperature": "14°C"}`},
	}

	for _, testCase := range testingData {
		// Call the function
		weatherData, err := getWeatherData(testCase.city, testRedisClient, "")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		expected := testCase.weatherData
		if weatherData != expected {
			t.Errorf("Expected %s, got %s", expected, weatherData)
		}
	}
}

func TestRootHandler_Get(t *testing.T) {
	tests := []struct {
		route     string
		filename  string
		expectErr bool
	}{
		{"/", "index.html", false},
	}

	for _, tt := range tests {

		// Create a request to pass to our handler.
		req, err := http.NewRequest("GET", tt.route, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Create a ResponseRecorder to record the response.
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rootHandler(w, r, nil, os.Getenv("WEATHER_API_KEY")) // Pass nil for Redis in this test
		})

		// Call the handler with the request and response recorder.
		handler.ServeHTTP(rr, req)

		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		f, err := os.Open(tt.filename)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		contents, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}

		expected := string(contents)
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
		}
	}
}

// Test for RootHandler POST method
func TestRootHandler_Post(t *testing.T) {
	// Create a request with the city parameter.
	form := url.Values{}
	form.Add("city", "London")
	req, err := http.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rootHandler(w, r, nil, os.Getenv("WEATHER_API_KEY")) // Pass nil for Redis in this test
	})

	// Call the handler with the request and response recorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check that the response body contains the expected content (you may want to adjust this)
	expected := "City: London \nWeather Data: " // Adjust according to your actual data
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want to contain %v", rr.Body.String(), expected)
	}
}

// Dummy function for testing purposes
func TestRootHandler_Post_ValidInputs(t *testing.T) {
	tests := []struct {
		city      string
		expectErr bool
	}{
		{"London", false},         // Valid input
		{"San Francisco", false},  // Valid input with space
		{"São Paulo", false},      // Valid input with special character
		{"Tokyo", false},          // Simple valid input
		{"   New York   ", false}, // Valid input with spaces
		{"", true},                // Empty input
		{" ", true},               // Input with only spaces
		{"@#$%^&*", true},         // Special characters
		{"ThisIsAnExtremelyLongCityNameThatExceedsNormalLengthLimits", true}, // Too long input
		{"<script>alert('test');</script>", true},                            // Input wiht script tags
		{"Boston2", false},      // City with a number
		{"Москва", false},       // City with non-latin characters
		{"New York 123", false}, // City with numbers
		{"O'Fallon", false},     // City with punctuation
	}

	for _, tt := range tests {
		t.Run(tt.city, func(t *testing.T) {
			form := url.Values{}
			form.Add("city", tt.city)
			req, err := http.NewRequest("POST", "/", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rootHandler(w, r, nil, "test_api_key") // Pass nil for Redis in this test
			})

			handler.ServeHTTP(rr, req)

			if tt.expectErr {
				if status := rr.Code; status != http.StatusBadRequest {
					t.Errorf("Expected status bad request; got %v", rr.Code)
				}
			} else {
				if status := rr.Code; status != http.StatusOK {
					t.Errorf("Expected status OK; got %v", rr.Code)
				}
			}
		})
	}
}
