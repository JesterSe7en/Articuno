package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
)

func main() {

	// Get the API key from the environment variable
	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set the WEATHER_API_KEY environment variable")
		os.Exit(1)
	}

	url := "https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/"

	fmt.Println(url)

	http.HandleFunc("/", RootHandler)
	fmt.Println("Listening on port localhost:8080")
	http.ListenAndServe(":8080", nil)
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("index.html") // load the html template
	if err != nil {
		fmt.Println(err)
	}
	tmpl.Execute(w, nil)
}
