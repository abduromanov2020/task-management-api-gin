package config

import (
	"errors"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port     string `envconfig:"PORT" default:"8080"`
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
	Env      string `envconfig:"ENV" default:"development"`

	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
	DBMaxConns  int32  `envconfig:"DB_MAX_CONNS" default:"20"`
	DBMinConns  int32  `envconfig:"DB_MIN_CONNS" default:"2"`

	JWTSecret   string        `envconfig:"JWT_SECRET" required:"true"`
	JWTTTL      time.Duration `envconfig:"JWT_TTL" default:"15m"`
	JWTIssuer   string        `envconfig:"JWT_ISSUER" default:"tasks-api"`
	JWTAudience string        `envconfig:"JWT_AUDIENCE" default:"tasks-api"`

	CORSOrigins      []string `envconfig:"CORS_ORIGINS" default:"http://localhost:3000"`
	AuthRateLimitRPM int      `envconfig:"AUTH_RATE_LIMIT_RPM" default:"5"`
}

func Load() (Config, error) {
	_ = godotenv.Load()

	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return Config{}, err
	}
	if len(c.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be at least 32 bytes")
	}
	c.Env = strings.ToLower(c.Env)
	return c, nil
}

func (c Config) IsProduction() bool { return c.Env == "production" }
