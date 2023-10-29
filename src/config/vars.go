package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	CLIENT_ID	string 
	CLIENT_SECRET	string 
	REDIRECT_URI string
	REDIS_URI string
	REDIS_PASSWORD string
	REDIS_DB int
)

func init() {
	// load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		// handle error
		log.Fatal("cannot load .env file")
	}

	// retrieve the environment variables and store them in package-level variables
	CLIENT_ID = os.Getenv("CLIENT_ID")
	CLIENT_SECRET = os.Getenv("CLIENT_SECRET")
	REDIRECT_URI = os.Getenv("REDIRECT_URI")
	REDIS_URI = os.Getenv("REDIS_URI")
	REDIS_PASSWORD = os.Getenv("REDIS_PASSWORD")
	redisDbInt, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil { 
		log.Fatal("cannot read Redis connection params")
	}
	REDIS_DB = redisDbInt
}
