package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"cloud.google.com/go/compute/metadata"
	"github.com/lietu/godometer/server"
)

const fakeProjectId = "some-fake-project-id"

var (
	fakeData  = flag.Bool("fakeData", false, "Generate fake data, for testing frontend. Optionally use the FAKE_DATA environment variable.")
	dev       = flag.Bool("dev", false, "Development mode (allow insecure traffic). Optionally use the DEV environment variable.")
	host      = flag.String("host", "0.0.0.0", "Which TCP address to listen on, 0.0.0.0 for all. Optionally use the HOST environment variable.")
	port      = flag.Int("port", 8080, "Which TCP port to listen to. Optionally use the PORT environment variable.")
	apiAuth   = flag.String("apiAuth", "", "Password for API. Optionally use the API_AUTH environment variable.")
	projectId = flag.String("projectId", fakeProjectId, "Google Cloud Project ID for Firestore access. Optionally use the PROJECT_ID environment variable.")
)

type Config struct {
	dev        bool
	fakeData   bool
	host       string
	projectId  string
	port       int
	apiAuth    string
	inCloudRun bool
}

func (c *Config) loadMetadata() {
	projectId, err := metadata.ProjectID()
	if err != nil {
		log.Printf("Error fetching project ID from metadata service: %s", err)
	} else {
		c.projectId = projectId
	}
}

func parseConfig() Config {
	flag.Parse()

	c := Config{
		fakeData:   *fakeData,
		dev:        *dev,
		host:       *host,
		projectId:  *projectId,
		port:       *port,
		apiAuth:    *apiAuth,
		inCloudRun: false,
	}

	if e := os.Getenv("DEV"); e != "" {
		if e == "1" || e == "yes" || e == "true" {
			c.dev = true
		} else {
			c.dev = false
		}
	}

	if e := os.Getenv("FAKE_DATA"); e != "" {
		if e == "1" || e == "yes" || e == "true" {
			c.dev = true
		} else {
			c.dev = false
		}
	}

	if e := os.Getenv("HOST"); e != "" {
		c.host = e
	}

	if e := os.Getenv("PORT"); e != "" {
		i, err := strconv.Atoi(e)
		if err != nil {
			log.Printf("Could not parse PORT environment variable: %s", err)
		} else {
			c.port = i
		}
	}

	if e := os.Getenv("API_AUTH"); e != "" {
		c.apiAuth = e
	}

	if e := os.Getenv("PROJECT_ID"); e != "" {
		c.projectId = e
	}

	// Try to automatically determine project ID when necessary
	if c.projectId == fakeProjectId {
		if e := os.Getenv("PORT"); e != "" {
			if e := os.Getenv("K_SERVICE"); e != "" {
				if e := os.Getenv("K_REVISION"); e != "" {
					if e := os.Getenv("K_CONFIGURATION"); e != "" {
						c.loadMetadata()
					}
				}
			}
		}
	}

	return c
}

func (c Config) Print() {
	pwd := "Not set"
	if c.apiAuth != "" {
		pwd = "Set"
	}

	log.Print(" ----- CONFIGURATION ----- ")
	log.Printf("Development:  %t", c.dev)
	log.Printf("Listen host:  %s", c.host)
	log.Printf("Listen port:  %d", c.port)
	log.Printf("Project ID:   %s", c.projectId)
	log.Printf("API password: %s", pwd)
}

func main() {
	config := parseConfig()

	if !config.dev {
		if config.apiAuth == "" {
			print("Not in development mode and no API password set. Aborting.")
			os.Exit(1)
		}
		if config.projectId == fakeProjectId {
			print("Not in development mode, and no Project ID set. Aborting.")
			os.Exit(1)
		}
	}

	srv := server.NewServer(config.dev, config.projectId, config.apiAuth)
	srv.Run(fmt.Sprintf("%s:%d", config.host, config.port), config.fakeData)
}
