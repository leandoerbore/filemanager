package filemanager

type Config struct {
	BindAddr  string `toml:"bind_addr"`
	LogLevel  string `toml:"log_level"`
	Endpoint  string `toml:"endpoint"`
	SecretKey string `toml:"secret_key"`
	AccessKey string `toml:"access_key"`
}

func NewConfig() *Config {
	return &Config{
		BindAddr: ":8080",
		LogLevel: "debug",
	}
}
