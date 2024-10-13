package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

var testRedisClient *redis.Client
var server = &http.Server{
	Addr: ":8080",
}
var sigChan = make(chan os.Signal, 1)
var serverReady = make(chan struct{})

func setup() error {

	// Initialize the Redis client
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

	// Start the web server in a goroutine
	go func() {
		// Signal readiness when the server starts accepting connections
		mux := http.NewServeMux()
		mux.HandleFunc("/health-check", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		server.Handler = mux

		close(serverReady) // Signal that the server is ready to handle requests

		if err := startWebServer(server, testRedisClient, os.Getenv("WEATHER_API_KEY")); err != nil {
			log.Fatalf("Failed to start web server: %v", err)
		}
	}()

	// Wait for server readiness before returning
	<-serverReady
	log.Println("Web server ready.")
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
		req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost%s%s", server.Addr, tt.route), nil)
		if err != nil {
			t.Fatal(err)
		}

		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Check the status code is what we expect.
		if status := resp.StatusCode; status != http.StatusOK {
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

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		expected := string(contents)
		if string(body) != expected {
			t.Errorf("handler returned unexpected body")
		}
	}
}

// Test for RootHandler POST method
func TestRootHandler_Post(t *testing.T) {
	// Create a request with the city parameter.
	form := url.Values{}
	form.Add("city", "London")
	url := fmt.Sprintf("http://localhost%s?%s", server.Addr, form.Encode())
	req, err := http.NewRequest("POST", url, nil)
	req.Form = form
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Check the status code is what we expect.
	if status := resp.StatusCode; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.FailNow()
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the response body contains the expected content (you may want to adjust this)
	expected := "City: London \nWeather Data: " // Adjust according to your actual data
	if !strings.Contains(string(body), expected) {
		t.Errorf("handler returned unexpected body")
	}
}

// Dummy function for testing purposes
func TestRootHandler_Post_ValidInputs(t *testing.T) {
	tests := []struct {
		city      string
		expectErr uint
	}{
		{"London", http.StatusOK},         // Valid input
		{"San Francisco", http.StatusOK},  // Valid input with space
		{"São Paulo", http.StatusOK},      // Valid input with special character
		{"Tokyo", http.StatusOK},          // Simple valid input
		{"   New York   ", http.StatusOK}, // Valid input with spaces
		{"", http.StatusNotFound},         // Empty input
		{" ", http.StatusNotFound},        // Input with only spaces
		{"@#$%^&*", http.StatusNotFound},  // Special characters
		{"ThisIsAnExtremelyLongCityNameThatExceedsNormalLengthLimits", http.StatusNotFound}, // Too long input
		{"<script>alert('test');</script>", http.StatusNotFound},                            // Input wiht script tags
		{"Boston2", http.StatusOK},      // City with a number
		{"Москва", http.StatusOK},       // City with non-latin characters
		{"New York 123", http.StatusOK}, // City with numbers
		{"O'Fallon", http.StatusOK},     // City with punctuation
	}

	for _, tt := range tests {
		t.Run(tt.city, func(t *testing.T) {
			req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost%s", server.Addr), strings.NewReader("city="+tt.city))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			if err != nil {
				t.Fatal(err)
			}

			client := http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if tt.expectErr != uint(resp.StatusCode) {
				t.Errorf("Expected status %v; got %v", tt.expectErr, resp.StatusCode)
			}
		})
	}
}
