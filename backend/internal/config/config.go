package config

import (
	"os"
)

type DBConfig struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
}

type HTTPConfig struct {
	Port string
}

type JWTConfig struct {
	Secret string
}

func LoadDB() DBConfig {
	cfg := DBConfig{
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
	}

	return cfg
}

func LoadHTTP() HTTPConfig {
	cfg := HTTPConfig{
		Port: os.Getenv("HTTP_PORT"),
	}

	return cfg
}

func LoadJWT() JWTConfig {
	cfg := JWTConfig{
		Secret: os.Getenv("JWT_SECRET"),
	}

	return cfg
}
