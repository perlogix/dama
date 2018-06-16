package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/go-redis/redis"
	"github.com/jinzhu/configor"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gin-contrib/secure"
	"github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	client  *docker.Client
	db      *redis.Client
	pwd     string
	version string
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var err error
	// Load server configurations from config.go and config.yml
	err = configor.Load(&DamaConfig, "config.yml")
	if err != nil {
		panic(err)
	}
	// Setup docker client
	client, err = docker.NewClient(DamaConfig.Docker.EndPoint)
	if err != nil {
		panic(err)
	}
	// Setup Redis DB client
	db = redis.NewClient(&redis.Options{
		Network:    DamaConfig.DB.Network,
		Addr:       DamaConfig.DB.Address,
		Password:   DamaConfig.DB.Password,
		DB:         DamaConfig.DB.DB,
		MaxRetries: DamaConfig.DB.MaxRetries,
	})
	_, err = db.Ping().Result()
	if err != nil {
		panic(err)
	}
	detectImg()
	pwd, _ = os.Getwd()

	go cleanContainers()

	if !DamaConfig.HTTPS.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	logger, _ := zap.NewProduction()
	secureConfig := secure.New(secure.Config{
		SSLRedirect:           true,
		STSSeconds:            315360000,
		STSIncludeSubdomains:  true,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: "default-src 'self'",
		IENoOpen:              true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		SSLProxyHeaders:       map[string]string{"X-Forwarded-Proto": "https"},
	})
	r.GET("/favicon.ico", gin.Recovery(), secureConfig)
	r.Use(gin.Recovery(), ginzap.Ginzap(logger, time.RFC3339, false), secureConfig)
	r.GET("/api/*name", api)
	r.POST("/api/*name", api)
	r.POST("/create-user", createUser)
	r.GET("/expire", expire)
	r.GET("/images", func(c *gin.Context) {
		imgs := map[string][]string{"images": DamaConfig.Images}
		c.JSON(200, imgs)
	})
	r.GET("/update", func(c *gin.Context) {
		c.String(200, version)
	})

	auth := r.Use(gin.BasicAuth(getAccounts()))
	auth.GET("/ws", ws)
	auth.GET("/api-name", getAPI)
	auth.POST("/create", create)
	auth.POST("/deploy", deploy)
	auth.POST("/uploads", uploads)
	auth.GET("/download", download)
	auth.POST("/envs", envs)

	// Set http server timeouts and idle connections
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 200
	s := &http.Server{
		Addr:           DamaConfig.HTTPS.Listen + ":" + DamaConfig.HTTPS.Port,
		Handler:        r,
		ReadTimeout:    120 * time.Second,
		WriteTimeout:   600 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	if _, err := os.Stat(DamaConfig.HTTPS.Pem); os.IsNotExist(err) {
		fmt.Println("https pem/cert doesn't not exist")
	}
	if _, err := os.Stat(DamaConfig.HTTPS.Pem); os.IsNotExist(err) {
		fmt.Println("https key doesn't not exist")
	}

	fmt.Println("dama is sponsored by TaskFit.io, built on " + version)
	s.ListenAndServeTLS(DamaConfig.HTTPS.Pem, DamaConfig.HTTPS.Key)
}
