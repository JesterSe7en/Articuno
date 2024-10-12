package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
)

// Global variables for Redis client and context
var rdb *redis.Client
var ctx = context.Background()

func globalSetup() error {
	if rdb != nil {
		return fmt.Errorf("Attempted to setup global redis client twice")
	}

	redis_url := os.Getenv("REDIS_URL")
	redis_password := os.Getenv("REDIS_PASSWORD")
	if redis_url == "" || redis_password == "" {
		return fmt.Errorf("Please set the REDIS_URL and REDIS_PASSWORD environment variables")
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     redis_url,
		Password: redis_password,
		DB:       0,
	})

	testingData := []struct {
		city        string
		weatherData string
	}{
		{"London", `{"location": "London", "temperature": "15°C"}`},
		{"Paris", `{"location": "Paris", "temperature": "18°C"}`},
		{"Berlin", `{"location": "Berlin", "temperature": "13°C"}`},
		{"Amsterdam", `{"location": "Amsterdam", "temperature": "14°C"}`},
	}

	pipe := rdb.Pipeline()

	for _, data := range testingData {
		pipe.Set(ctx, data.city, data.weatherData, 0)
	}
	cmds, err := pipe.Exec(ctx)

	if err != nil {
		return err
	}

	// check individual errors
	for _, cmd := range cmds {
		if cmd.Err() != nil {
			return cmd.Err()
		}
	}
	return nil
}
func globalTeardown() {
	if rdb != nil {
		rdb.Close()
	}
}

func TestMain(m *testing.M) {
	err := globalSetup()
	if err != nil {
		fmt.Println("Test setup failed: ", err)
		globalTeardown()
		os.Exit(1)
	}

	defer globalTeardown()

	fmt.Println("Test setup successful; running tests...")
	code := m.Run()

	os.Exit(code)
}

func TestGetWeatherData(t *testing.T) {
	// Just use existing Redis since developing on Windows rn
	// Ideally use a local Redis instance
	redis_url := os.Getenv("REDIS_URL")
	redis_password := os.Getenv("REDIS_PASSWORD")
	rdb := redis.NewClient(&redis.Options{
		Addr:     redis_url, // Use existing Redis instance
		Password: redis_password,
	})

	// Mock API response
	city := "London"

	// Assume you have a way to mock the weather API response
	// This is a simple implementation just for testing purposes
	mockWeatherData := `{"location": "London", "temperature": "15°C"}`
	res, err := rdb.Set(context.Background(), city, mockWeatherData, 0).Result()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if res != "OK" {
		t.Errorf("Expected 'OK', got %s", res)
	}

	// Call the function
	weatherData, err := getWeatherData(city, rdb, "")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	expected := mockWeatherData
	if weatherData != expected {
		t.Errorf("Expected %s, got %s", expected, weatherData)
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
