package main

import (
	"crypto/tls"
	"io"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/gin-gonic/gin"
	"github.com/taskfitio/dama/data"
	"github.com/yhat/wsutil"
)

// User struct is to read JSON values into struct to create new users
type User struct {
	Username string `json:"username"`
	Token    string `json:"token"`
	Role     string `json:"role"`
}

// api route is proxy requests to the right container based on name http param
func api(c *gin.Context) {
	name := c.Param("name")
	var api string
	if dpAPI, _ := db.HGet("deployedPort", name).Result(); dpAPI != "" {
		api = "localhost:" + dpAPI
	}
	if api == "" {
		if sbAPI, _ := db.HGet("sandboxPort", name).Result(); sbAPI != "" {
			api = "localhost:" + sbAPI
		}
	}
	if api == "" {
		c.String(400, "API not found")
		return
	}
	backendURL := &url.URL{Scheme: "http", Host: api}
	p := httputil.NewSingleHostReverseProxy(backendURL)
	var pathRewrite string
	pathRewrite = strings.TrimPrefix(c.Request.URL.Path, "/api/"+name)
	if pathRewrite == "" {
		pathRewrite = "/"
	}
	c.Request.URL.Path = pathRewrite
	p.ServeHTTP(c.Writer, c.Request)
}

//TODO: Make POST
// expire route is to increate the expire value for a user. Default expire from config.yml is set.
func expire(c *gin.Context) {
	user := c.Query("user")
	expire := c.Query("expire")
	if expire == "" || user == "" {
		c.String(400, "Bad request")
		return
	}
	if exp, _ := db.HGet(user, "expire").Result(); exp != "" {
		db.HSet(user, "expire", expire)
		c.String(201, "Updated expire")
	}
}

// createUser route is used to create a new user into Redis DB
func createUser(c *gin.Context) {
	usr := &User{}
	if err := c.Bind(usr); err != nil {
		c.String(500, err.Error())
		return
	}
	if exist, _ := db.HGet("accounts", usr.Username).Result(); exist != "" {
		c.String(400, "User already in DB")
		return
	}
	user := map[string]interface{}{
		"key":      usr.Token,
		"sandbox":  genToken(),
		"deployed": genToken(),
		"expire":   Config.Expire,
		"role":     usr.Role,
	}
	accounts := map[string]interface{}{
		usr.Username: usr.Token,
	}
	var err error
	_, err = db.HMSet("accounts", accounts).Result()
	if err != nil {
		c.String(500, "Error adding to accounts")
		return
	}
	_, err = db.HMSet(usr.Username, user).Result()
	if err != nil {
		c.String(500, "Error adding user")
		return
	}
	db.BgSave()
	getAccounts()
	c.String(201, "Created")
}

// getAPI route is used to retrieve keys for sandbox and deploy APIs
func getAPI(c *gin.Context) {
	name := c.MustGet(gin.AuthUserKey).(string)
	deployAPI, _ := db.HGet(name, "deployed").Result()
	sandAPI, _ := db.HGet(name, "sandbox").Result()
	if deployAPI != "" && sandAPI != "" {
		c.String(200, deployAPI+":"+sandAPI)
		return
	}
	c.String(404, "")
}

// ws route is used to proxy ws & wss to the correct running gotty docker container
func ws(c *gin.Context) {
	name := c.MustGet(gin.AuthUserKey).(string)
	file := c.Request.Header.Get("File")
	new := c.Request.Header.Get("New")
	image := c.Request.Header.Get("Image")
	port := c.Request.Header.Get("Port")
	var wsPort string
	var backend string
	if new != "" {
		deleteContainers(name, "build")
	}
	wsPort, _ = db.HGet("wsPort", name).Result()
	if new == "" && wsPort != "" {
		backend = "localhost:" + wsPort
	} else {
		ctr, err := createContainer(name, image, file, port, false)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		sbAPI, _ := db.HGet(name, "sandbox").Result()
		db.HSet("sandboxPort", sbAPI, strings.Split(ctr, ":")[1])
		wsPort = strings.Split(ctr, ":")[0]
		db.HSet("wsPort", name, wsPort)
		backend = "localhost:" + wsPort
	}
	var scheme string
	if Config.Gotty.TLS {
		scheme = "wss"
	} else {
		scheme = "ws"
	}
	backendURL := &url.URL{Scheme: scheme, Host: backend}
	p := wsutil.NewSingleHostReverseProxy(backendURL)
	p.TLSClientConfig = &tls.Config{InsecureSkipVerify: Config.HTTPS.VerifyTLS}
	p.ServeHTTP(c.Writer, c.Request)
}

// deploy route is called when user requests a new deploy from their dama.yml
func deploy(c *gin.Context) {
	name := c.MustGet(gin.AuthUserKey).(string)
	df := &data.Damafile{}
	if err := c.Bind(df); err != nil {
		c.String(500, err.Error())
		return
	}
	if df.Image != "" && !checkImg(df.Image) {
		c.String(404, df.Image+" Image not found")
		return
	}
	path := pwd + "/upload/" + name
	os.MkdirAll(path, 0755)
	t := template.New("tmpl")
	t, _ = t.Parse(tmpl)
	f, _ := os.Create(path + "/.dama")
	os.Chmod(path+"/.dama", 0755)
	t.Execute(f, df)
	f.Close()
	if df.Env != nil {
		var envs = make(map[string]interface{})
		for _, e := range df.Env {
			split := strings.Split(e, "=")
			envs[split[0]] = split[1]
		}
		db.HMSet(name+"_env", envs)
	}
	file := path + "/.dama"
	image := df.Image
	var port string
	if df.Port == "" {
		port = "5000"
	} else {
		port = df.Port
	}
	var ctr string
	ctr, err := createContainer(name, image, file, port, true)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	deployed, _ := db.HGet(name, "deployed").Result()
	db.HSet("deployedPort", deployed, strings.Split(ctr, ":")[1])
	c.String(201, deployed)
}

// uploads route is for uploading files to the users workspace directory
func uploads(c *gin.Context) {
	name := c.MustGet(gin.AuthUserKey).(string)
	c.Request.ParseMultipartForm(32 << 20)
	path := pwd + "/upload/" + name
	pathSize, err := dirSize(path)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	if pathSize >= int64(Config.UploadSize) {
		c.String(500, "Workspace size limit reached")
		return
	}
	info, handler, err := c.Request.FormFile("uploadfile")
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer info.Close()

	file := path + "/" + filepath.Base(handler.Filename)

	out, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0640)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer out.Close()
	io.Copy(out, info)
	c.String(201, "Uploaded")
}

// download route is for downloading files from workspace directory
func download(c *gin.Context) {
	name := c.MustGet(gin.AuthUserKey).(string)
	file := c.Query("file")
	if file == "" {
		c.String(500, "No file specified")
		return
	}
	path := pwd + "/upload/" + name + "/" + file
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		c.String(500, err.Error())
		return
	}
	c.File(path)
}

// envs route is for setting environment variables for running docker containers
func envs(c *gin.Context) {
	name := c.MustGet(gin.AuthUserKey).(string)
	env := &data.Damafile{}
	if err := c.Bind(env); err != nil {
		c.String(500, err.Error())
		return
	}
	if env.Env == nil {
		c.String(400, "No environment settings")
		return
	}
	for _, e := range env.Env {
		if match, _ := regexp.MatchString("\\w*=\\w*", e); !match {
			c.String(500, "Environment setting needs to be key=value")
			return
		}
	}
	envs, _ := db.HGetAll(name + "_env").Result()
	totalMapLen := len(env.Env) + len(envs)
	if totalMapLen < Config.EnvSize {
		var envs = make(map[string]interface{})
		for _, e := range env.Env {
			split := strings.Split(e, "=")
			envs[split[0]] = split[1]
		}
		db.HMSet(name+"_env", envs)
		c.String(201, "Created")
		return
	}
	c.String(500, "Reached max amount of env tags, max is 20")
}

// create route is called when creating a new on-demand or sandbox container
func create(c *gin.Context) {
	name := c.MustGet(gin.AuthUserKey).(string)
	df := &data.Damafile{}
	if err := c.Bind(df); err != nil {
		c.String(500, err.Error())
		return
	}
	if df.Image != "" && !checkImg(df.Image) {
		c.String(404, df.Image+" Image not found")
		return
	}
	path := pwd + "/upload/" + name
	os.MkdirAll(path, 0755)
	t := template.New("tmpl")
	t, _ = t.Parse(tmpl)
	f, _ := os.Create(path + "/.dama")
	os.Chmod(path+"/.dama", 0755)
	t.Execute(f, df)
	f.Close()

	if df.Env != nil {
		var envs = make(map[string]interface{})
		for _, e := range df.Env {
			split := strings.Split(e, "=")
			envs[split[0]] = split[1]
		}
		db.HMSet(name+"_env", envs)
	}
	c.String(201, "OK")
}
