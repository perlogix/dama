package main

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/satori/go.uuid"

	docker "github.com/fsouza/go-dockerclient"
)

// genToken generates a UUID key for sandbox & deploy when a new account is created
func genToken() string {
	id := uuid.NewV4()
	token := strings.Split(id.String(), "-")
	return token[4]
}

// checkImg checks to make sure docker image requested to run is on host
func checkImg(a string) bool {
	for _, b := range DamaConfig.Images {
		if b == a {
			return true
		}
	}
	return false
}

// getAccounts is used to load accounts into BasicAuth gin middleware
func getAccounts() gin.Accounts {
	accounts, _ := db.HGetAll("accounts").Result()
	if len(accounts) == 0 {
		if DamaConfig.AdminUsername != "" && DamaConfig.AdminPassword != "" {
			accounts = map[string]string{DamaConfig.AdminUsername: DamaConfig.AdminPassword}
		}
	}
	return accounts
}

// getCmd is used to append dama script to docker cmd slice
func getCmd(img string) []string {
	inspectImg, _ := client.InspectImage(img)
	imgCmd := inspectImg.Config.Cmd
	lastCmd := imgCmd[len(imgCmd)-1]
	if lastCmd == "bash" || lastCmd == "sh" || lastCmd == "/bin/bash" || lastCmd == "/bin/sh" {
		imgCmd = imgCmd[:len(imgCmd)-1]
		damaScript := []string{"/bin/bash", "-c", "/root/workspace/.dama"}
		imgCmd = append(imgCmd, damaScript...)
	}
	return imgCmd
}

// createContainer creates container for sandbox or deployed environment
func createContainer(name, image, file, port string, deploy bool) (string, error) {
	var cmd []string
	var binds []string
	var img string
	var hostname string
	labels := make(map[string]string)
	var env []string
	envs, err := db.HGetAll(name + "_env").Result()
	if err == nil {
		for k, v := range envs {
			env = append(env, k+"="+v)
		}
	}

	env = append(env, "USER="+name)
	labels["dama"] = "dama"
	labels["user"] = name
	if image == "" {
		img = DamaConfig.Images[0]
	} else {
		img = image
	}

	if port == "" {
		port = "5000"
	}

	if deploy {
		cmd = []string{"/bin/bash", "/root/workspace/.dama"}
		labels["expire"] = DamaConfig.DeployExpire
		labels["API"] = "true"
		env = append(env, "API=true")
		deleteContainers(name, "API")
		deployedAPI, _ := db.HGet(name, "deployed").Result()
		hostname = deployedAPI
	} else {
		var expire string
		expire, err = db.HGet(name, "expire").Result()
		if err != nil {
			expire = DamaConfig.Expire
		}
		labels["expire"] = expire
		labels["build"] = "true"
		sandboxAPI, _ := db.HGet(name, "sandbox").Result()
		hostname = sandboxAPI
		deleteContainers(name, "build")
		if file != "" {
			cmd = getCmd(img)
		}
	}
	uploadPath := filepath.Clean(pwd + "/upload/" + name)
	binds = []string{uploadPath + ":/root/workspace:rw"}
	var portBindings = map[docker.Port][]docker.PortBinding{}
	portStr := docker.Port(port + "/tcp")
	portBindings = map[docker.Port][]docker.PortBinding{
		"8080/tcp": []docker.PortBinding{docker.PortBinding{HostIP: "0.0.0.0"}},
		portStr:    []docker.PortBinding{docker.PortBinding{HostIP: "0.0.0.0"}},
	}
	hostConfig := &docker.HostConfig{PublishAllPorts: false, PortBindings: portBindings, Privileged: false, Binds: binds}
	opts := docker.CreateContainerOptions{Config: &docker.Config{CPUShares: DamaConfig.Docker.CPUShares, Memory: DamaConfig.Docker.Memory, Cmd: cmd, Hostname: hostname, Image: img, Labels: labels, Env: env, ExposedPorts: map[docker.Port]struct{}{portStr: {}, "8080/tcp": {}}}, HostConfig: hostConfig}
	ctr, err := client.CreateContainer(opts)
	if err != nil {
		return "", err
	}
	err = client.StartContainer(ctr.ID, hostConfig)
	if err != nil {
		return "", err
	}
	insp, err := client.InspectContainer(ctr.ID)
	if err != nil {
		return "", err
	}

	ws := insp.NetworkSettings.Ports["8080/tcp"][0].HostPort
	api := insp.NetworkSettings.Ports[portStr][0].HostPort
	return ws + ":" + api, nil
}

// deleteContainers is used to delete container if new flag or run is specified
func deleteContainers(user, label string) {
	ctrs, _ := client.ListContainers(docker.ListContainersOptions{All: true, Filters: map[string][]string{"label": {"dama"}}})
	for _, ctr := range ctrs {
		for k, v := range ctr.Labels {
			if k == "user" && v == user {
				for k := range ctr.Labels {
					if k == label {
						client.RemoveContainer(docker.RemoveContainerOptions{ID: ctr.ID, Force: true})
					}
				}
			}
		}
	}
}

// cleanContainers is ran in background via goroutine to clean up expired containers
func cleanContainers() {
	for {
		ctrs, _ := client.ListContainers(docker.ListContainersOptions{All: true, Filters: map[string][]string{"label": {"dama"}}})
		for _, ctr := range ctrs {
			for k, v := range ctr.Labels {
				if k == "expire" {
					created, err := client.InspectContainer(ctr.ID)
					if err == nil {
						delta := time.Now().Sub(created.Created)
						expireInt, _ := strconv.Atoi(v)
						if int(delta.Seconds()) > expireInt {
							for k := range ctr.Labels {
								if k == "build" {
									db.HDel("wsPort", ctr.Labels["user"])
								}
							}
							client.RemoveContainer(docker.RemoveContainerOptions{ID: ctr.ID, Force: true})
						}
					}
				}
				status := path.Base(strings.Split(ctr.Status, " ")[0])
				if status == "Exited" {
					client.RemoveContainer(docker.RemoveContainerOptions{ID: ctr.ID, Force: true})
				}
			}
		}
		time.Sleep(time.Second * 10)
	}
}

// stringInSlice is to check if string exists in slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// detectImg is used when starting up to check if all docker images in config.yml are present on host
func detectImg() {
	dkrList, err := client.ListImages(docker.ListImagesOptions{All: true})
	if err != nil {
		panic(err)
	}
	var dkrImgs []string
	for _, img := range dkrList {
		if len(img.RepoTags) != 0 {
			dkrImgs = append(dkrImgs, img.RepoTags[0])
		}
	}
	for _, img := range DamaConfig.Images {
		if !stringInSlice(img, dkrImgs) {
			panic(img + " Image not found")
		}
	}
}

// dirSize checks to see if upload directory has reached it's max size before uploading new files
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
