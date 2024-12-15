package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	natting "github.com/docker/go-connections/nat"
	dbc "github.com/go-mysql-org/go-mysql/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

var (
	Version = "0.0.1"
	ApiHost = "https://api.wpx.dev"
)

type AddonType string

const (
	PluginAddon AddonType = "Plugin"
	ThemeAddon  AddonType = "Theme"
)

var (
	AddonMatchKVReg = regexp.MustCompile(`^(.*):\s(.*)$`)
	AddonNameReg    = regexp.MustCompile(`^(Theme|Plugin)\sName:\s(.*)$`)
)

type Addon struct {
	Slug string     `json:"slug"`
	Info *AddonInfo `json:"info"`
}

type AddonInfo struct {
	Type            AddonType `json:"type"`
	Name            string    `key:"${Type} Name" json:"name"`
	Author          string    `key:"Author" json:"author"`
	AuthorURI       string    `key:"Author URI" json:"author_uri"`
	UpdateURI       string    `key:"Update URI" json:"update_uri"`
	Description     string    `key:"Description" json:"description"`
	Version         string    `key:"Version" json:"version"`
	RequiresAtLeast string    `key:"Requires at least" json:"requires_at_least"`
	TestedUpTo      string    `key:"Tested up to"  json:"tested_up_to"`
	RequiresPHP     string    `key:"Requires PHP"  json:"requires_php"`
	License         string    `key:"License"  json:"licence"`
	LicenseURI      string    `key:"License URI"  json:"license_uri"`
	TextDomain      string    `key:"Text Domain"  json:"text_domain"`
}

func main() {

	if os.Getenv("WPXDEV") == "true" {
		ApiHost = "http://localhost:4200"
	}

	cmd := &cli.Command{
		Name:  "wpx",
		Usage: "WordPress Development Lifecycle CLI",
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Display version of this program",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					fmt.Println(Version)
					return nil
				},
			},
			{
				Name:  "check",
				Usage: "Check required dependinces",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					checkHasGit()
					checkHasDocker()
					return nil
				},
			},
			{
				Name:  "test",
				Usage: "Test current plugin or theme within docker",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name: "path",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {

					var err error
					var path = cmd.String("path")

					if path == "" {
						path, err = os.Getwd()
						if err != nil {
							panic(err)
						}
					}

					path, err = filepath.Abs(path)

					if err != nil {
						return err
					}

					if err := runAddonFromPath(path); err != nil {
						panic(err)
					}

					return nil
				},
			},
			{
				Name:  "create",
				Usage: "Create new plugin or theme",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "type",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "slug",
						Required: true,
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					return nil
				},
			},
			{
				Name:    "login",
				Aliases: []string{"t"},
				Usage:   "Login to the cloud for deployments",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "user",
						Required:    true,
						DefaultText: "login",
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {

					fmt.Print("Enter Password: ")
					password, err := term.ReadPassword(int(syscall.Stdin))
					if err != nil {
						panic(err)
					}

					fmt.Print("\n")

					fmt.Println("Authenticating...")

					if err := auth(c.String("user"), string(password)); err != nil {
						fmt.Printf("%s\n", err.Error())
						os.Exit(1)
					}

					fmt.Println("Authentification successfully")

					return nil
				},
			},
		},
	}

	createWorkspace()

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Println(err)
	}
}

func checkHasGit() {
	fmt.Print("[WPX] ")
	cmd := exec.Command("git", "version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Git missing")
		os.Exit(1)
	}
	fmt.Print(strings.ToUpper(string(output)))
}

func checkHasDocker() {
	fmt.Print("[WPX] ")
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		fmt.Println("Docker missing")
		os.Exit(1)
	}
	defer apiClient.Close()

	fmt.Printf("DOCKER VERSION: %s\n", apiClient.ClientVersion())
}

func auth(user, password string) error {

	payload := bytes.NewBufferString(
		fmt.Sprintf(
			`{"user":"%s", "password":"%s"}`,
			user, password,
		),
	)

	resp, err := http.Post(ApiHost+"/auth", "application/json", payload)

	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("Authentication failed")
	}

	data, err := io.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	setSetting("auth.key", string(data))

	return nil
}

// todo: choose container to run into
func runAddonFromPath(path string) error {

	_, err := checkAddonInfoFromPath(path)

	if err != nil {
		panic(err)
	}

	slug := filepath.Base(path)

	credentials := strings.ReplaceAll(slug, "-", "")

	group := "instance"

	if err := provideDatabaseCredentials(group, credentials, credentials); err != nil {
		return err
	}

	docker, err := client.NewClientWithOpts(client.FromEnv)

	if err != nil {
		return errors.Wrap(err, "Can't connect to docker")
	}

	rc, err := docker.ImagePull(
		context.TODO(),
		"wordpress",
		image.PullOptions{},
	)

	if err != nil {
		return errors.Wrap(err, "Can't pull wordpress image")
	}

	// todo: use a buffer
	io.Copy(os.Stdin, rc)

	newport, err := natting.NewPort("tcp", "80")
	if err != nil {
		return errors.Wrap(err, "Can't create docker wordpress container port")
	}

	var wpconfextr string

	if os.Getenv("WPXDEV") == "true" {

		wpconfextr = `
			define('WPX_CORE_DEV', true);
		`

		fmt.Println("Set WPX_CORE_DEV true")
	}

	createContainerResponse, err := docker.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: "wpxi:latest",
			Env: []string{
				"WORDPRESS_DB_HOST=db",
				"WORDPRESS_DB_USER=" + credentials,
				"WORDPRESS_DB_PASSWORD=" + credentials,
				"WORDPRESS_DB_NAME=wp_" + group,
				"WORDPRESS_DEBUG=1",
				"WORDPRESS_CONFIG_EXTRA=" + wpconfextr,
			},
		},
		&container.HostConfig{
			PortBindings: natting.PortMap{
				newport: []natting.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: "80",
					},
				},
			},
			NetworkMode: "wpx",
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
			Mounts: []mount.Mount{
				mount.Mount{
					Type:        mount.TypeBind,
					Source:      path,
					Target:      "/var/www/html/wp-content/plugins/" + slug,
					Consistency: mount.ConsistencyDefault,
					BindOptions: &mount.BindOptions{
						Propagation:      mount.PropagationPrivate,
						CreateMountpoint: true,
					},
				},
			},
		},
		&network.NetworkingConfig{},
		&v1.Platform{},
		slug,
	)

	if err != nil {
		return errors.Wrap(err, "Can't create container")
	}

	if err := docker.ContainerStart(
		context.Background(),
		createContainerResponse.ID,
		container.StartOptions{},
	); err != nil {
		return err
	}

	containerExec, err := docker.ContainerExecCreate(
		context.Background(),
		createContainerResponse.ID,
		container.ExecOptions{
			Tty:          true,
			User:         "root",
			Privileged:   true,
			AttachStdin:  true,
			AttachStderr: true,
			AttachStdout: true,
			WorkingDir:   "/var/www/html",
			Cmd: []string{
				"/usr/local/bin/wp",
				"plugin",
				"activate",
				slug,
				"--allow-root",
			},
		},
	)

	if err != nil {
		return err
	}

	err = docker.ContainerExecStart(
		context.Background(),
		containerExec.ID,
		container.ExecStartOptions{},
	)

	if err != nil {
		return err
	}

	openURL(
		fmt.Sprintf("http://localhost:%s", newport.Port()),
	)

	return nil
}

func parseAddonInfo(file string) (*AddonInfo, error) {

	data, err := os.Open(file)

	if err != nil {
		return nil, err
	}

	ai := AddonInfo{}

	scan := bufio.NewReader(data)

	var insideComment bool

	for {

		line, _, err := scan.ReadLine()

		if err != nil {
			break
		}

		if strings.Trim(string(line), " ") == "/*" {
			insideComment = true
			continue
		}

		if insideComment && strings.Trim(string(line), " ") == "*/" {
			insideComment = false
			continue
		}

		if insideComment {

			if AddonMatchKVReg.Match(line) {

				matches := AddonMatchKVReg.FindStringSubmatch(string(line))

				if AddonNameReg.Match(line) {
					matches = AddonNameReg.FindStringSubmatch(string(line))
					ai.Type = AddonType(matches[1])
					ai.Name = strings.Trim(matches[2], " ")
					continue
				}

				aitof := reflect.TypeOf(ai)
				aivof := reflect.ValueOf(&ai)

				for i := 0; i < aitof.NumField(); i++ {
					if v, ok := aitof.Field(i).Tag.Lookup("key"); ok && v == matches[1] {
						aivof.Elem().Field(i).SetString(strings.Trim(matches[2], " "))
					}
				}
			}
		}
	}

	return &ai, nil
}

func checkAddonInfoFromPath(root string) (ai *AddonInfo, err error) {

	fi, err := os.Stat(root)

	if err != nil {
		return nil, errors.New("path not found: " + root)
	}

	if !fi.IsDir() {
		return nil, errors.New("expected a directory path")
	}

	if ai, err := parseAddonInfo(root + "style.css"); ai != nil {
		fmt.Println(err)
		return ai, nil
	}

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if d != nil && !d.IsDir() {
			if addonInfo, _ := parseAddonInfo(path); addonInfo != nil {
				ai = addonInfo
				return errors.New("found")
			}
		}
		return nil
	})

	if ai == nil {
		return nil, errors.New("no addon defined in path")
	}

	return
}

// run and create database for instance
func provideDatabaseCredentials(group, username, password string) error {

	docker, err := client.NewClientWithOpts(client.FromEnv)

	if err != nil {
		return errors.Wrap(err, "Can't connect to docker")
	}

	var dbv volume.Volume

	var databaseVolumeID = getSettings().Database.Volume

	if databaseVolumeID == "" {
		dbv, err = docker.VolumeCreate(
			context.Background(),
			volume.CreateOptions{
				Name: "wpx_mysql_database",
			},
		)
	} else {
		dbv, err = docker.VolumeInspect(
			context.Background(),
			databaseVolumeID,
		)
	}

	if err != nil {
		return errors.Wrap(err, "Can't verify volume for database")
	}

	rc, err := docker.ImagePull(
		context.TODO(),
		"docker.io/library/mysql",
		image.PullOptions{},
	)

	if err != nil {
		return errors.Wrap(err, "Can't pull mysql image")
	}

	// todo: use a buffer
	io.Copy(os.Stdin, rc)

	docker.NetworkCreate(
		context.TODO(),
		"wpx",
		network.CreateOptions{},
	)

	var databaseContainer container.CreateResponse
	var newport natting.Port

	if getSettings().Database.Container != "" {

		dbc, err := docker.ContainerInspect(
			context.TODO(),
			getSettings().Database.Container,
		)

		if err != nil {
			goto CREATE_DATABASE_CONTAINER
		}

		if !dbc.State.Running {
			if err := docker.ContainerStart(
				context.TODO(),
				dbc.ID,
				container.StartOptions{},
			); err != nil {
				return errors.Wrap(err, "Can't start container")
			}
		}

		goto MYSQL_CREATE_DATABASE
	}

CREATE_DATABASE_CONTAINER:

	newport, err = natting.NewPort("tcp", "3306")
	if err != nil {
		return errors.Wrap(err, "Can't create docker port")
	}

	databaseContainer, err = docker.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: "mysql",
			Labels: map[string]string{
				"com.docker.compose.project": "wpx",
			},
			Env: []string{
				"MYSQL_ROOT_PASSWORD=password",
			},
		},
		&container.HostConfig{
			PortBindings: natting.PortMap{
				newport: []natting.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: "3306",
					},
				},
			},
			NetworkMode: "wpx",
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
			Binds: []string{
				fmt.Sprintf("%s:/var/lib/mysql", dbv.Mountpoint),
			},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				"wpx": &network.EndpointSettings{
					Aliases: []string{"db"},
				},
			},
		},
		&v1.Platform{},
		"wpx-database",
	)

	if err != nil {
		return errors.Wrap(err, "Can't create database container")
	}

	setSetting("database.container", databaseContainer.ID)

	if err := docker.ContainerStart(
		context.TODO(),
		databaseContainer.ID,
		container.StartOptions{},
	); err != nil {
		return errors.Wrap(err, "Can't start database container")
	}

MYSQL_CREATE_DATABASE:

	spawnDatabaseAdmin()

	var tries = 0
	var maxTries = 10

MYSQL_CONNECT:

	tries++
	fmt.Printf("Creating database (try %d)...\n", tries)
	conn, err := dbc.Connect("127.0.0.1:3306", "root", "password", "")

	if err != nil {
		if tries > maxTries {
			return errors.Wrap(err, "Can't create database")
		}
		time.Sleep(time.Second)
		goto MYSQL_CONNECT
	}

	if err := conn.Ping(); err != nil {
		if tries > maxTries {
			return errors.Wrap(err, "Can't create database")
		}
		time.Sleep(time.Second)
		goto MYSQL_CONNECT
	}

	fmt.Printf("Try access %s:%s for group %s\n", username, password, group)

	res, err := conn.Execute(
		fmt.Sprintf(`SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = 'wp_%s'`, group),
	)

	if err != nil {
		return errors.Wrap(err, "Can't check if database exists")
	}

	if res.RowNumber() == 0 {

		fmt.Println("Create database...")

		if _, err := conn.Execute(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS wp_%s", group)); err != nil {
			return errors.Wrap(err, "Can't run create database sql")
		}

	} else {
		fmt.Println("Database already created...")
	}

	conn.Execute("use mysql")

	res, err = conn.Execute(
		fmt.Sprintf(`select  User, Host from  user where User like '%s'`, username),
	)

	if err != nil {
		return errors.Wrap(err, "Can't check if username exists")
	}

	if res.RowNumber() == 0 {

		fmt.Println("Create privilege...")

		if _, err := conn.Execute(fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED BY '%s'", username, password)); err != nil {
			return errors.Wrap(err, "Can't run create user sql")
		}

		if _, err := conn.Execute(fmt.Sprintf("GRANT ALL PRIVILEGES ON wp_%s.* TO '%s'@'%%'", group, username)); err != nil {
			return errors.Wrap(err, "Can't run grant privileges sql")
		}

		if _, err := conn.Execute("FLUSH PRIVILEGES"); err != nil {
			return errors.Wrap(err, "Can't run flush privileges sql")
		}

	} else {
		fmt.Println("Privilege already created...")
	}

	if err := conn.Close(); err != nil {
		return errors.Wrap(err, "Can't close mysql connection")
	}

	return nil
}

func spawnDatabaseAdmin() {

	docker, err := client.NewClientWithOpts(client.FromEnv)

	if err != nil {
		fmt.Println("Can't connect to docker daemon")
		os.Exit(1)
	}

	rc, err := docker.ImagePull(
		context.TODO(),
		"docker.io/library/adminer",
		image.PullOptions{},
	)

	if err != nil {
		panic(err)
	}

	io.Copy(os.Stdin, rc)

	newport, err := natting.NewPort("tcp", "8080")
	if err != nil {
		fmt.Println("Unable to create docker port")
		panic(err)
	}

	databaseAdminerContainer, err := docker.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: "adminer",
			Labels: map[string]string{
				"com.docker.compose.project": "wpx",
			},
		},
		&container.HostConfig{
			PortBindings: natting.PortMap{
				newport: []natting.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: "8080",
					},
				},
			},
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				"wpx": &network.EndpointSettings{
					Links: []string{"db"},
				},
			},
		},
		&v1.Platform{},
		"wpx-adminer",
	)

	if err == nil {
		if err := docker.ContainerStart(
			context.TODO(),
			databaseAdminerContainer.ID,
			container.StartOptions{},
		); err != nil {
			panic(err)
		}
	}
}

func createWorkspace() {

	homePath := os.Getenv("HOME")

	configDirPath := homePath + "/.wpx"

	fi, err := os.Stat(configDirPath)

	if err != nil {

		if err := os.Mkdir(configDirPath, os.ModePerm); err != nil {
			panic(err)
		}

		return
	}

	if !fi.IsDir() {
		if err := os.Mkdir(configDirPath, os.ModePerm); err != nil {
			panic(err)
		}
	}
}

type ProjectSettings struct {
	Container string `json:"container"`
}

type Settings struct {
	Database struct {
		Container string `json:"container"`
		Volume    string `json:"volume"`
	} `json:"database"`

	Projects map[string]ProjectSettings `json:"projects"`
}

func getDefaultSettings() *Settings {
	return &Settings{}
}

func getSettings() *Settings {

	homePath := os.Getenv("HOME")

	data, err := os.ReadFile(homePath + "/.wpx/config.json")

	if err != nil {
		return getDefaultSettings()
	}

	settings := &Settings{}

	if err := json.Unmarshal(data, settings); err != nil {
		fmt.Println(err)
		return getDefaultSettings()
	}

	return settings
}

func setSetting(path, value string) {

	homePath := os.Getenv("HOME")

	data, err := os.ReadFile(homePath + "/.wpx/config.json")

	if err != nil {
		configString, err := sjson.Set("", path, value)
		if err != nil {
			panic(err)
		}

		if err := os.WriteFile(homePath+"/.wpx/config.json", []byte(configString), os.ModePerm); err != nil {
			panic(err)
		}
	}

	configString, err := sjson.Set(string(data), path, value)

	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(homePath+"/.wpx/config.json", []byte(configString), os.ModePerm); err != nil {
		panic(err)
	}
}

func openURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // "linux", "freebsd", "openbsd", "netbsd"
		// Check if running under WSL
		if isWSL() {
			// Use 'cmd.exe /c start' to open the URL in the default Windows browser
			cmd = "cmd.exe"
			args = []string{"/c", "start", url}
		} else {
			// Use xdg-open on native Linux environments
			cmd = "xdg-open"
			args = []string{url}
		}
	}
	if len(args) > 1 {
		// args[0] is used for 'start' command argument, to prevent issues with URLs starting with a quote
		args = append(args[:1], append([]string{""}, args[1:]...)...)
	}
	return exec.Command(cmd, args...).Start()
}

// isWSL checks if the Go program is running inside Windows Subsystem for Linux
func isWSL() bool {
	releaseData, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(releaseData)), "microsoft")
}
