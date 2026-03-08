package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type Host struct {
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	MAC            string   `yaml:"mac"`
	IP             string   `yaml:"ip"`
	User           string   `yaml:"user" json:"-"`
	Password       string   `yaml:"password" json:"-"`
	KeyPath        string   `yaml:"key_path" json:"-"`
	Cmd            string   `yaml:"cmd" json:"-"`
	SkipInterfaces []string `yaml:"skip_interfaces" json:"-"`
}

// load hosts from yaml file, this is where you add new hosts to manage, along with their credentials and shutdown commands
func LoadHosts() ([]Host, error) {
	fmt.Println("Attempting to load hosts from data/hosts.yaml...")
	data, err := os.ReadFile("data/hosts.yaml")
	if err != nil {
		fmt.Printf("Error reading data/hosts.yaml: %v\n", err)
		return nil, err
	}
	fmt.Println("Successfully read data/hosts.yaml file.")

	var hosts []Host
	err = yaml.Unmarshal(data, &hosts)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

func findHost(id string) (*Host, error) {
	hosts, err := LoadHosts()
	if err != nil {
		return nil, err
	}
	for i := range hosts {
		if hosts[i].ID == id {
			return &hosts[i], nil
		}
	}
	return nil, fmt.Errorf("host %s not found", id)
}

func main() {
	fmt.Println("Main Starting...")
	var err error = nil
	//http server for API
	r := gin.Default()

	r.Use(cors.Default())

	//woprk in progress, example api
	r.GET("/api/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	//API to get the list of hosts, useful for the frontend
	r.GET("/api/hosts", func(c *gin.Context) {
		hosts, _ := LoadHosts()
		if err != nil {
			// Se c'è un errore, rispondiamo con un codice 500
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Errore nel caricamento host"})
			return
		}
		c.JSON(http.StatusOK, hosts)
	})

	//API Wake-on-LAN
	r.POST("/api/wol/:id", func(c *gin.Context) {
		id := c.Param("id")

		target, err := findHost(id)
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}

		if err := SendWol(target); err != nil {
			c.JSON(500, gin.H{"error": "WoL Failed: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"message": "Magic Packet sent successfully to " + target.Name})
	})

	//API Shutdown
	r.POST("/api/shutdown/:id", func(c *gin.Context) {
		id := c.Param("id")

		target, err := findHost(id)
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}

		err = RemoteShutdown(target)

		c.JSON(200, gin.H{"message": "Shutdown command received from " + target.Name})
	})

	// Serve static files from the React frontend "dist" folder
	r.Static("/assets", "/app/frontend/dist/assets")
	r.StaticFile("/vite.svg", "/app/frontend/dist/vite.svg")
	r.LoadHTMLGlob("/app/frontend/dist/index.html")

	// Catch-all route for React Router (if you ever use it) or just to serve the main HTML page
	r.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Get port from environment variable, default to 8080 if not set
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
	log.Fatal(err)
}
