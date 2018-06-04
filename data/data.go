package data

// This package is shared between server and CLI

// AWS_S3 configuration for AWS_S3 primary key
type AWS_S3 struct {
	File       string `yaml:"file" json:"file"`
	Dir        string `yaml:"dir" json:"dir"`
	BucketPush string `yaml:"bucket_push" json:"bucket_push"`
	BucketPull string `yaml:"bucket_pull" json:"bucket_pull"`
}

// Git configurations for git primary key
type Git struct {
	URL    string `yaml:"url" json:"url"`
	Branch string `yaml:"branch" json:"branch"`
	SHA    string `yaml:"sha" json:"sha"`
}

// Damafile struct for both server JSON & client YML
type Damafile struct {
	Project    string   `yaml:"project" json:"project"`
	Env        []string `yaml:"env" json:"env"`
	Checkout   string   `yaml:"checkout" json:"checkout"`
	TimeFormat string   `yaml:"time_format" json:"time_format"`
	SetupCmd   string   `yaml:"setup_cmd" json:"setup_cmd"`
	Cmd        string   `yaml:"cmd" json:"cmd"`
	Python     string   `yaml:"python" json:"python"`
	Pip        string   `yaml:"pip" json:"pip"`
	Image      string   `yaml:"image" json:"image"`
	Port       string   `yaml:"port" json:"port"`
	Git        Git
	AWS_S3     AWS_S3
}
