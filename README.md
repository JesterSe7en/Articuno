# Weather API 🌦️

**Weather API** is a simple web server built in Go that fetches weather data from the [Visual Crossing Weather API](https://www.visualcrossing.com/weather-api) and presents it in a user-friendly HTML format. The server allows users to query weather data by city or ZIP code (USA only) and implements caching with Redis to optimize repeated queries.

This project is a demonstration of backend development skills, API integration, caching mechanisms, and the ability to create simple, effective web applications.

---

## Project Overview

This project demonstrates:
- **API Integration**: Fetches real-time weather data using the Visual Crossing Weather API.
- **Web Server in Go**: Uses Go's `net/http` package to handle incoming requests and serve HTML.
- **Form Handling**: Allows users to input their city or ZIP code, with data validation.
- **Redis Caching**: Implements Redis to cache weather data for previously queried locations, improving performance and minimizing redundant API calls.
  
---

## Features 🌟

1. **Real-Time Weather Data**: 
   - Queries weather data for a given city or ZIP code from [Visual Crossing's API](https://www.visualcrossing.com/weather-api).
   - Displays temperature, conditions, and other essential weather details in a simple, clean HTML page.

2. **User-Friendly Form**:
   - Accepts city or ZIP code input from the user to search for weather data.
   - Data validation ensures that only valid inputs are processed.

3. **Efficient Caching with Redis**:
   - Redis is used to cache responses for repeated queries by the same city or ZIP code, improving performance.
   - Caching helps avoid redundant API calls, reducing response times and API costs.

4. **Scalability**: 
   - The project can be easily extended to add more features such as additional weather data, support for international locations, or integration with other third-party services.

---

## Tech Stack 💻

- **Go**: Backend web server using the standard `net/http` library to handle routing and form submissions.
- **Redis**: Caching layer to store and quickly retrieve weather data for repeated queries.
- **HTML**: Simple HTML form for user input and data presentation.
- **Visual Crossing Weather API**: Provides real-time weather data for various locations.

---

## Redis Caching Strategy

- Cached data is stored for 1 hour to ensure weather data is relatively fresh.
- After the time expires, the server fetches new data from the Visual Crossing API.
- Redis automatically will remove expired data, ensuring only recent weather queries are cached.

---

## Installation 🛠️

1. **Clone the repository**:
   ```powershell
   git clone https://github.com/yourusername/weather_api.git
   cd weather_api

2. **Set up environment variables**:  Make sure to set the `WEATHER_API_KEY`, `REDIS_PASSWORD`, and `REDIS_URL` environment variables.
   ```powershell
   $env:WEATHER_API_KEY = "your_api_key"
   $env:REDIS_PASSWORD = "your_redis_password"
   $env:REDIS_URL = "localhost:6379"
3. **Install Redis (if not installed already)**: For this project, a Redis docker container hosted on a raspberry pi was used to cache the weather data.
   ```bash
   sudo apt-get install redis-server
   # optional to check the redis instance
   sudo apt-get install redis-cli
4. **Configure Redis container**:
   ```yaml
   version: '3.8'
   services:
   redis:
      image: redis:latest
      restart: always
      ports:
         - '6379:6379'
      command: redis-server --save 60 1 --loglevel warning --requirepass <some_redis_password>
      volumes:
         - ./data:/data
5. **Build the project**:
   ```powershell
   go build -o weather_api main.go
6. **Run the project**:
   ```powershell
   ./weather_api
7. **Access the web application**: Open your browser and go to `http://localhost:8080` to access the weather form.

## Testing 🧪

Basic unit tests are provided in the `main_test.go` file.
The unit tests cover:
- **API calls**: Ensuring the correct data is retrieved from the Visual Crossing API.
- **Cache validation**: Ensuring that cached data is correctly retrieved and invalidated after the set time period.
- **Form validation**: Checking that the input validation works as expected (for both valid and invalid inputs).

To run them, run `go test` from the root directory.


## Usage 📝

1. **Input a city name or ZIP code**: Navigate to the web page and enter either a city name or a ZIP code in the form.

2. **View the weather data**: The server will fetch the data from the Visual Crossing API (or Redis if cached) and display it on the HTML page.

3. **Redis Cache**: If the city or ZIP code has been queried recently, the response will come from Redis to minimize API calls.

## Why This Project? 🤔

I built this project to highlight some of my backend engineering skills, focusing on:

- **API Integration**: Learning to work with third-party APIs to fetch real-time weather data.
- **Web Development in Go**: Creating a lightweight web server that’s quick and efficient.
- **Caching with Redis**: Using Redis to optimize performance and reduce API calls.
- **User Input Handling**: Making sure user input is validated and processed smoothly.
- **Scalability**: Structuring the code for future growth and improvements.
- **Unit Testing**: Adding some basic tests to keep everything running smoothly.
- **Documentation**: Writing clear instructions to help others understand how to use the project.

## Acknowledgements 🙏

- [Visual Crossing Weather API](https://www.visualcrossing.com/weather-api)
- [Redis](https://redis.io/)
- [Go](https://go.dev/)
- [Roadmap.sh](https://roadmap.sh/)
