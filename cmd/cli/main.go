package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	json "github.com/json-iterator/go"
	"github.com/leekchan/timeutil"
	"github.com/perlogix/dama/data"
	gottyclient "github.com/perlogix/data/gotty-client"
	"github.com/ryanuber/columnize"
	"golang.org/x/net/http2"
	"gopkg.in/yaml.v2"
)

var (
	username string
	key      string
	version  string
	img      string
	server   string
	c        *http.Client
	strf     = "%Y%m%d%I%M%S"
	usage    = `Usage: dama [options] <args>

 -new           Create a new environment from scratch and delete the old one
 -run           Create environment and run with dama.yml or script
 -file          Run with dama.yml in different directory
 -env           Create an environment variable or secret for runtime
 -img           Specify a docker image to be used instead of the default image
 -dl            Download file from workspace in your environment to your local computer
 -up            Upload files from your local computer to workspace in your environment
 -deploy        Deploy API and get your unique URI
 -show-api      Show API details: URL, Health and Type
 -show-images   Show images available to use

`
)

// gitRev try to get git revision from HEAD
func gitRev() string {
	gitCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	gitOut, err := gitCmd.Output()
	if err != nil {
		return ""
	}
	return string(gitOut)
}

// postEnv is used to post new environment variables to server
func postEnv(e string) (string, error) {
	env := data.Damafile{Env: strings.Split(e, ",")}
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(env)
	req, err := http.NewRequest("POST", server+"envs", b)
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	req.SetBasicAuth(username, key)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// postCreate is used to create new container to connect with websocket
func postCreate(t data.Damafile) (string, error) {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(t)
	req, err := http.NewRequest("POST", server+"create", b)
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	req.SetBasicAuth(username, key)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		return "", errors.New("Could not create environment")
	}
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body), nil
}

// postFiles is used to post files to upload route of server
func postFiles(filename string) error {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	fileWriter, err := bodyWriter.CreateFormFile("uploadfile", filename)
	if err != nil {
		return err
	}

	fh, err := os.Open(filename)
	if err != nil {
		return err
	}

	_, err = io.Copy(fileWriter, fh)
	if err != nil {
		return err
	}

	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	url := server + "uploads"
	req, err := http.NewRequest("POST", url, bodyBuf)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", contentType)
	req.SetBasicAuth(username, key)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(bodyBytes))
	}

	return nil
}

// downloadFile is used to download file from workspace on server
func downloadFile(filepath string) error {
	url := server + "download?file=" + filepath
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(username, key)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("File does not exist")
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// deployAPI is used to create a new deployed container
func deployAPI(t data.Damafile) (string, error) {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(t)
	req, err := http.NewRequest("POST", server+"deploy", b)
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	req.SetBasicAuth(username, key)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		return "", errors.New(string(body))
	}
	return string(body), nil
}

// tryAPI is used to call web service in running container when deploying
func tryAPI(url string) string {
	time.Sleep(time.Second * 10)
	for i := 0; i < 10; i++ {
		resp, _ := c.Get(url)
		if resp.StatusCode < 500 {
			return "API is served at " + url
		}
		time.Sleep(time.Second * 10)
	}
	return "API is still deploying will be served at " + url
}

// apiDetails is used to get health of running web services for sandox and deploy API
func apiDetails(name string) string {
	var (
		deployURL     string
		deployHealth  string
		sandboxURL    string
		sandboxHealth string
	)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	cl := &http.Client{
		Timeout:   1 * time.Second,
		Transport: tr,
	}

	req, err := http.NewRequest("GET", server+"api-name", nil)
	if err != nil {
		return err.Error()
	}
	req.SetBasicAuth(username, key)

	var resp *http.Response

	resp, err = c.Do(req)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "Error retrieving API details"
		}
		deployURL = server + "api/" + strings.Split(string(body), ":")[0]
		sandboxURL = server + "api/" + strings.Split(string(body), ":")[1]
	}

	if sandboxURL != "" {
		resp, _ = cl.Get(sandboxURL)
		if resp.StatusCode != 400 && resp.StatusCode < 500 {
			sandboxHealth = "online"
		} else {
			sandboxHealth = "offline"
		}
	}

	if deployURL != "" {
		resp, _ = cl.Get(deployURL)
		if resp.StatusCode != 400 && resp.StatusCode < 500 {
			deployHealth = "online"
		} else {
			deployHealth = "offline"
		}
	}
	var output []string
	output = []string{"URL | HEALTH | TYPE", sandboxURL + "|" + sandboxHealth + "|" + "sandbox", deployURL + "|" + deployHealth + "|" + "deployed"}
	return columnize.SimpleFormat(output)
}

// images struct is used in imgDetails to unmarshal JSON for images available on server
type images struct {
	Images []string `json:"images"`
}

// imgDetails makes a request to server to get all available docker images from server
func imgDetails() string {
	url := server + "images"
	resp, _ := c.Get(url)
	var imgs images
	if resp.StatusCode == 200 {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "Error retrieving images details"
		}
		json.Unmarshal(body, &imgs)
	} else {
		return "Error retrieving images details"
	}

	output := []string{"NAME"}
	output = append(output, imgs.Images...)
	return columnize.SimpleFormat(output)
}

func main() {
	run := flag.Bool("run", false, "New container")
	file := flag.String("file", "", "File location")
	new := flag.Bool("new", false, "New environment")
	env := flag.String("env", "", "New environment variable")
	img := flag.String("img", "", "Specify image")
	dl := flag.String("dl", "", "Download file")
	upload := flag.String("up", "", "Upload file")
	deploy := flag.Bool("deploy", false, "Deploy API")
	showAPI := flag.Bool("show-api", false, "Show API details")
	showImgs := flag.Bool("show-images", false, "Show image details")
	flag.Usage = func() {
		fmt.Println(usage)
	}
	flag.Parse()
	username = os.Getenv("DAMA_USER")
	key = os.Getenv("DAMA_KEY")
	if username == "" {
		fmt.Println("DAMA_USER is not set in your env")
		os.Exit(1)
	}
	if key == "" {
		fmt.Println("DAMA_KEY is not set in your env")
		os.Exit(1)
	}
	damaEnv := os.Getenv("DAMA_SERVER")
	if _, err := url.Parse(damaEnv); err == nil && damaEnv != "" {
		server = damaEnv
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		TLSHandshakeTimeout: time.Second * 30,
	}

	http2.ConfigureTransport(tr)

	c = &http.Client{
		Transport: tr,
		Timeout:   time.Second * 600,
	}

	if *showImgs {
		fmt.Println(imgDetails())
		os.Exit(0)
	}

	if *showAPI {
		fmt.Println(apiDetails(key))
		os.Exit(0)
	}

	if *dl != "" {
		err := downloadFile(*dl)
		if err != nil {
			fmt.Println(err)
			os.Exit(0)
		}
		fmt.Println("Download complete " + *dl)
		os.Exit(0)
	}

	if *upload != "" {
		err := postFiles(*upload)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Upload complete " + *upload)
		os.Exit(0)
	}

	if *env != "" {
		resp, err := postEnv(*env)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(resp)
		os.Exit(0)
	}
	cli, err := gottyclient.NewClient(server)
	if err != nil {
		fmt.Println("Environment no longer available, try\ndama -new")
		os.Exit(1)
	}
	f := data.Damafile{}
	var port string
	match, _ := regexp.MatchString("dama.yml$", *file)
	_, err = os.Stat("./dama.yml")
	if *run && *file == "" || *run && *file != "" || match && os.IsNotExist(err) || *deploy {
		var rFile []byte
		var filep string
		if match {
			filep = *file
		} else {
			filep = "./dama.yml"
		}
		rFile, err = ioutil.ReadFile(filep)
		if err == nil {
			e := yaml.Unmarshal(rFile, &f)
			if e != nil {
				fmt.Println(e.Error())
				os.Exit(1)
			}
		}
		if f.Git.SHA == "" {
			f.Git.SHA = strings.TrimSpace(gitRev())
		}
		if f.Port == "" {
			port = "5000"
		} else {
			port = f.Port
		}
		if *img == "" {
			*img = f.Image
		}
		if f.Project == "" {
			wd, _ := os.Getwd()
			f.Project = filepath.Base(wd)
		}
		t := time.Now()
		if f.TimeFormat == "" {
			f.Env = append(f.Env, "TIMESTAMP="+timeutil.Strftime(&t, strf))
		} else {
			f.Env = append(f.Env, "TIMESTAMP="+timeutil.Strftime(&t, f.TimeFormat))
		}
		f.Env = append(f.Env, "PROJECT="+f.Project)
		pipSplit := strings.Split(f.Pip, "\n")
		pipJoin := strings.Join(pipSplit, " ")
		f.Pip = pipJoin
		if *deploy {
			uri, err := deployAPI(f)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			fmt.Println(tryAPI(server + "api/" + uri))
			os.Exit(0)
		}
		postCreate(f)
		if err := cli.Loop(*run, true, "build", *img, username, key, port); err != nil {
			fmt.Println("Environment no longer available, try\ndama -new")
			os.Exit(1)
		}
		os.Exit(0)
	}
	if *run && *file != "" {
		if err := cli.Loop(*run, true, *file, *img, username, key, port); err != nil {
			fmt.Println("Environment no longer available, try\ndama -new")
			os.Exit(1)
		}
	}
	if *run && *file == "" {
		fmt.Println("Cannot do a run without script")
		fmt.Println(usage)
		os.Exit(1)
	}
	if !*run && *file != "" {
		fmt.Println("Cannot do a run without specifying -run option")
		fmt.Println(usage)
		os.Exit(1)
	}
	if err := cli.Loop(*run, *new, *file, *img, username, key, port); err != nil {
		fmt.Println("Environment no longer available, try\ndama -new")
		os.Exit(1)
	}
}
