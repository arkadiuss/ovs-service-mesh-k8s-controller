package config

type Config struct {
	ConsulAddr string
	VirtualIP  string
}

func GetConfig() *Config {
	return &Config{
		ConsulAddr: "http://localhost:8500",
		VirtualIP:  "10.1.1.254",
	}
}
