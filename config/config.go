package config

type Config struct {
	ListenAddr string `envconfig:"LISTEN_ADDR"`
}
