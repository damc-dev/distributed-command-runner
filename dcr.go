package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/aybabtme/rgbterm"
	"github.com/urfave/cli"
)

type Server struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
	Tags        Tags   `json:"tags"`
}

type Servers []Server

type Tags []string

func getServers(configFile string) Servers {
	raw, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	var servers []Server
	json.Unmarshal(raw, &servers)
	return servers
}

func filterByEnvironment(servers Servers, environment string) Servers {
	filtered := servers[:0]
	for _, server := range servers {
		if server.Environment == environment {
			filtered = append(filtered, server)
		}
	}
	return filtered
}

func contains(tags []string, expectedTag string) bool {
	for _, tag := range tags {
		if tag == expectedTag {
			return true
		}
	}

	return false
}

func filterByTag(servers Servers, tag string) Servers {
	filtered := servers[:0]
	for _, server := range servers {
		if strings.HasPrefix(tag, "!") {
			if !contains(server.Tags, strings.TrimPrefix(tag, "!")) {
				filtered = append(filtered, server)
			}
		} else {
			if contains(server.Tags, tag) {
				filtered = append(filtered, server)
			}
		}
	}
	return filtered
}

func listNamesOutput(servers Servers) {
	var buffer bytes.Buffer
	for index, server := range servers {
		buffer.WriteString(server.Name)
		if index < (len(servers) - 1) {
			buffer.WriteString(",")
		}
	}
	fmt.Println(buffer.String())
}

func printJSONOutput(servers Servers) {
	serversJSON, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(serversJSON))
}

func columnarOutput(servers Servers) {
	for _, server := range servers {
		fmt.Printf("%s %s %s\n", server.Environment, server.Name, server.Tags)
	}
}

func formatList(servers Servers, format string) {
	if format == "names" {
		listNamesOutput(servers)
	} else if format == "json" {
		printJSONOutput(servers)
	} else {
		columnarOutput(servers)
	}
}

func filterServers(servers Servers, environment string, tags []string) Servers {
	if environment != "" {
		servers = filterByEnvironment(servers, environment)
	}
	if tags != nil && len(tags) != 0 {
		for _, tag := range tags {
			servers = filterByTag(servers, tag)
		}
	}
	return servers
}

func execCommand(server Server, user string, command string) (exitCode int, stdout string, stderr string) {
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	exit := 0

	cmd := exec.Command("pmrun", "-h", server.Name, user, command)
	//cmd := exec.Command("echo", "Hello "+user)
	//cmd := exec.Command("ls", "Hello "+command)

	//fmt.Println(cmd)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	startErr := cmd.Start()
	if startErr != nil {
		log.Fatalf("cmd.Start: %v", startErr)
	}
	er := cmd.Wait()
	if er != nil {
		if exiterr, ok := er.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				exit = status.ExitStatus()
			}
		}
	}

	return exit, stdoutBuf.String(), stderrBuf.String()
}

func red() (r uint8, g uint8, b uint8) {
	return 255, 0, 0
}

func green() (r uint8, g uint8, b uint8) {
	return 0, 255, 0
}

func white() (r uint8, g uint8, b uint8) {
	return 255, 255, 255
}

func colorizeExitCode(exitCode int) string {
	fr, fg, fb := white()
	br, bg, bb := green()
	if exitCode != 0 {
		br, bg, bb = red()
	}
	return rgbterm.String(strconv.Itoa(exitCode), fr, fg, fb, br, bg, bb)
}

func main() {
	var configFile string
	var environment string
	var tags string
	var format string
	var user string

	app := cli.NewApp()
	app.Usage = "List and filter servers"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "config, c",
			Value:       os.Getenv("HOME") + "/.dcr/servers.json",
			Usage:       "Load configuration from `FILE`",
			Destination: &configFile,
		},
		cli.StringFlag{
			Name:        "env, e",
			Usage:       "Filter by environment",
			Destination: &environment,
		},
		cli.StringFlag{
			Name:        "tags, t",
			Usage:       "Filter by tags",
			Destination: &tags,
		},
	}

	app.Commands = []cli.Command{
		{
			Name:    "list",
			Aliases: []string{"l", "ls"},
			Usage:   "List servers",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "format, f",
					Usage:       "Output format",
					Destination: &format,
				},
			},
			Action: func(c *cli.Context) error {
				servers := getServers(configFile)
				servers = filterServers(servers, environment, strings.Split(tags, ","))
				formatList(servers, format)
				fmt.Println("")
				return nil
			},
		},
		{
			Name:    "exec",
			Aliases: []string{"x", "run"},
			Usage:   "Execute command",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "user, u",
					Usage:       "User to run as",
					Destination: &user,
				},
			},
			Action: func(c *cli.Context) error {
				cmd := c.Args().Get(0)
				servers := getServers(configFile)
				servers = filterServers(servers, environment, strings.Split(tags, ","))
				for _, server := range servers {
					exitCode, stdout, stderr := execCommand(server, user, cmd)
					fmt.Printf("\n%2s[%10s] STDOUT: %10s\n", colorizeExitCode(exitCode), server.Name, strings.Trim(stdout, "\n"))
					if stderr != "" {
						fmt.Printf("STDERR: %s\n", strings.Trim(stderr, "\n"))
					}
				}
				fmt.Println("")
				return nil
			},
		},
	}

	app.Run(os.Args)
}
