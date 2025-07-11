package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var Token string
var SocketPassword string
var GeminiApiKey string

type Config struct {
	Token          string
	SocketPassword string
	GeminiApiKey   string
	ServerIp	   string
}

func New() Config {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalln("Could not load .env file: ", err)
	}

	config := Config{
		Token:          os.Getenv("TOKEN"),
		SocketPassword: os.Getenv("SOCKET_PASSWORD"),
		GeminiApiKey:   os.Getenv("GEMINI_API_KEY"),
		ServerIp:		os.Getenv("SERVER_IP"),
	}

	return config
}
