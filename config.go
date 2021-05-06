package main

// Redis struct for redis primary key, contains redis configurations for redis server
type Redis struct {
	Network    string `required:"true"`
	Address    string `required:"true"`
	DB         int    `default:"0"`
	MaxRetries int    `default:"20"`
	Password   string `env:"DBPassword"`
}

// HTTPS struct for https primary key, contains https configurations for server and client
type HTTPS struct {
	Listen    string `default:"0.0.0.0"`
	Port      string `default:"8443"`
	Debug     bool   `default:"false"`
	Pem       string `required:"true"`
	Key       string `required:"true"`
	VerifyTLS bool   `default:"false"`
}

// Docker struct for docker primary key, contains docker client configurations
type Docker struct {
	EndPoint  string `default:"unix:///var/run/docker.sock"`
	CPUShares int64  `default:"512"`
	Memory    int64  `default:"1073741824"`
}

// Gotty struct for gotty primary key, contains gotty configurations
type Gotty struct {
	TLS bool `default:"false"`
}

// DamaConfig variable with config.yml configurations
var DamaConfig = struct {
	AdminUsername string   `env:"DamaUser" required:"true"`
	AdminPassword string   `env:"DamaPassword" required:"true"`
	Images        []string `required:"true"`
	Expire        string   `default:"1200"`
	DeployExpire  string   `default:"86400"`
	UploadSize    int      `default:"2000000000"`
	EnvSize       int      `default:"20"`
	Gotty         Gotty
	Docker        Docker
	DB            Redis
	HTTPS         HTTPS
}{}
