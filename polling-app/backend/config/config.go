package config

import "os"

type Config struct {
	MongoURI    string
	MongoDBName string
	RedisAddr   string
	RedisPass   string
	JWTSecret   string
	Port        string
	AllowOrigin string
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Load() *Config {
	return &Config{
		MongoURI:    getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDBName: getEnv("MONGO_DB", "pollingapp"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:   getEnv("REDIS_PASSWORD", ""),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-me"),
		Port:        getEnv("PORT", "8080"),
		AllowOrigin: getEnv("ALLOW_ORIGIN", "http://localhost:5173"),
	}
}
