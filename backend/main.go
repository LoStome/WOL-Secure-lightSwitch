package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type Host struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	MAC  string `yaml:"mac"`
	IP   string `yaml:"ip"`
	User           string   `yaml:"user" json:"-"`
	Password       string   `yaml:"password" json:"-"`
	KeyPath        string   `yaml:"key_path" json:"-"`
	Cmd            string   `yaml:"cmd" json:"-"`      
	SkipInterfaces []string `yaml:"skip_interfaces" json:"-"` 
}

//load hosts from yaml file, this is where you add new hosts to manage, along with their credentials and shutdown commands
func LoadHosts() ([]Host, error) {
	data, err := os.ReadFile("hosts.yaml")
	if err != nil {
		return nil, err
	}

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

    r.GET("/api/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "message": "pong",
        })
    })


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
        
        // Gestione speciale per l'EOF: se il server si spegne bruscamente, è successo!
        if err != nil && !strings.Contains(err.Error(), "EOF") {
            c.JSON(500, gin.H{"error": "Fallimento spegnimento: " + err.Error()})
            return
        }

        c.JSON(200, gin.H{"message": "Comando di spegnimento ricevuto da " + target.Name})
    })

    r.Run(":8080") 
	//

	var targetID = "server-proxmox" // Questo valore sarà dinamico con le API
	var action string =""
	//wol or shutdown, only for test env

	hosts, _ := LoadHosts()
	var target *Host
	
	//checks if target exists in the hosts list, if not it exits with an error
	for i := range hosts {
		if hosts[i].ID == targetID {
			target = &hosts[i]
			break
		}
	}
	if target == nil {
		log.Fatalf("Host %s not found", targetID)
	}

	
	switch action {
	case "wol":
		err = SendWol(target)

	case "shutdown":
		err = RemoteShutdown(target)
		
	default:
		err = fmt.Errorf("Invalid action: %s", action)
	}

	//checks errors and prints the result of the action
	if err != nil {
		fmt.Printf("Error during %s: %v\n", action, err)
		log.Printf("Error during %s: %v\n", action, err)
	} else {
		fmt.Printf("%s completed successfully for %s!\n", action, target.Name)
		log.Printf("%s completed successfully for %s!\n", action, target.Name)
	}
}