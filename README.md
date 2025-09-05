# Go Crawler Project

## Overview
This project implements a web crawler in Go that extracts real estate listings specifically for houses for sale from a predefined list of real estate websites. The extracted data is stored in a MongoDB database.

## Project Structure
```
go-crawler-project
├── api
│   ├── handler
│   │   └── property_handler.go
│   └── router.go
├── cmd
│   └── crawler
│       └── main.go
├── configs
│   └── sites.json
├── internal
│   ├── config
│   │   └── config.go
│   ├── crawler
│   │   └── crawler.go
│   ├── repository
│   │   └── mongo_repository.go
│   └── service
│       └── property_service.go
├── .env
├── Dockerfile
├── docker-compose.yaml
├── go.mod
├── Makefile
└── README.md
```

## Requirements
- Go (latest stable version)
- MongoDB

## Setup Instructions

### Environment Variables
Create a `.env` file in the root directory with the following variables:
```
MONGO_URI=<your_mongo_uri>
PORT=<service_port>
```

### Configuration
The list of URLs to crawl is defined in `configs/sites.json`. Update this file with the desired real estate websites.

### Running the Application
1. Build the application:
   ```
   make build
   ```
2. Run the application locally:
   ```
   make run
   ```

### Docker
To build and run the application using Docker:
1. Build the Docker image:
   ```
   docker build -t go-crawler .
   ```
2. Run the Docker container:
   ```
   docker-compose up
   ```

### Testing
To run the tests:
```
make test
```

### Deployment
To deploy the application to Google Cloud Run:
```
make deploy
```

## API Endpoints
- `GET /properties`: Retrieves all properties from the database.

## Logging
The application uses a structured logger for logging events and errors.

## Contributing
Feel free to submit issues or pull requests for improvements or bug fixes.