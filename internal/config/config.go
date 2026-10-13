package config

import "time"

type Config struct {
	RingTimeout     time.Duration
	LineRecoverWait time.Duration
}

func Default() Config {
	return Config{
		RingTimeout:     90 * time.Second,
		LineRecoverWait: 3 * time.Second,
	}
}
