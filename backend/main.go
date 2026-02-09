package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Host struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	MAC  string `yaml:"mac"`
	IP   string `yaml:"ip"`
	User string `yaml:"user"`
	Password       string   `yaml:"password"`
    KeyPath        string   `yaml:"key_path"`
	Cmd  string `yaml:"cmd"`
	SkipInterfaces []string `yaml:"skip_interfaces"`
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



func main() {
	fmt.Println("Main Starting...")

	var targetID = "server-proxmox" // Questo valore sarà dinamico con le API
	var action string ="shutdown"
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

	var err error = nil
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