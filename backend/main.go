package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
	PingInterval   int      `yaml:"ping_interval" json:"ping_interval"`
	Online         bool     `yaml:"-" json:"online"`
	LastPinged     string   `yaml:"-" json:"last_pinged"`
}

type HostState struct {
	Online     bool
	LastPinged string
}

var hostStates = struct {
	sync.RWMutex
	Status map[string]HostState
}{Status: make(map[string]HostState)}

// load hosts from yaml file, this is where you add new hosts to manage, along with their credentials and shutdown commands
func LoadHosts() ([]Host, error) {
	fmt.Println("Attempting to load hosts from data/hosts.yaml...")
	path := "data/hosts.yaml"
	data, err := os.ReadFile(path)
	if err != nil {
		path = "../data/hosts.yaml"
		data, err = os.ReadFile(path)
		if err != nil {
			fmt.Printf("Error reading hosts.yaml: %v\n", err)
			return nil, err
		}
	}
	// Uncomment for debug
	// fmt.Printf("Successfully read %s file.\n", path)

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

func StartPingManager() {
	fmt.Println("Ping Manager Started...")
	lastPingTimes := make(map[string]time.Time)

	for {
		hosts, err := LoadHosts()
		if err != nil {
			fmt.Printf("PingManager: Error loading hosts: %v\n", err)
			time.Sleep(10 * time.Second) // retry later
			continue
		}

		now := time.Now()
		for _, h := range hosts {
			interval := h.PingInterval
			if interval <= 0 {
				interval = 60 // Default to 60 seconds
			}

			lastPing, exists := lastPingTimes[h.ID]
			if !exists || now.Sub(lastPing).Seconds() >= float64(interval) {
				lastPingTimes[h.ID] = now
				go func(host Host) {
					online := IsOnline(host.IP)
					hostStates.Lock()
					state := hostStates.Status[host.ID]
					state.Online = online
					if online {
						state.LastPinged = time.Now().Format("15:04:05")
					}
					hostStates.Status[host.ID] = state
					hostStates.Unlock()
				}(h)
			}
		}

		time.Sleep(5 * time.Second) // check every 5 seconds if a ping should be triggered
	}
}

// ----------------- API Handlers -----------------

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := GetUserByEmail(req.Email)
	if err != nil {
		// If user not found, check if there are any admins. If not, auto-create this user as the first admin.
		hasAdmins, dbErr := HasAdmins()
		if dbErr == nil && !hasAdmins {
			hash, hashErr := HashPassword(req.Password)
			if hashErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
				return
			}
			err = CreateUser(req.Email, hash, true, []string{})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create initial admin user"})
				return
			}
			// Fetch the newly created user
			user, err = GetUserByEmail(req.Email)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve new admin user"})
				return
			}
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
	} else if !CheckPasswordHash(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	token, err := GenerateJWT(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"is_admin": user.IsAdmin,
		},
	})
}

// Check if user is authorized for a specific device based on UserDevice mapping
func isAuthorizedForDevice(userID uint, deviceID string, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	user, err := GetUserByID(userID)
	if err != nil {
		return false
	}
	for _, dev := range user.Devices {
		if dev.DeviceID == deviceID {
			return true
		}
	}
	return false
}

func handleGetHosts(c *gin.Context) {
	hosts, err := LoadHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error loading hosts"})
		return
	}

	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	// Filter hosts based on authorization
	var authorizedHosts []Host
	for i := range hosts {
		if isAuthorizedForDevice(userID, hosts[i].ID, isAdmin) {
			// Attach online state to hosts from cache
			hostStates.RLock()
			state := hostStates.Status[hosts[i].ID]
			hosts[i].Online = state.Online
			if state.LastPinged == "" {
				hosts[i].LastPinged = "N/A"
			} else {
				hosts[i].LastPinged = state.LastPinged
			}
			hostStates.RUnlock()
			
			authorizedHosts = append(authorizedHosts, hosts[i])
		}
	}

	c.JSON(http.StatusOK, authorizedHosts)
}

func handleWOL(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	if !isAuthorizedForDevice(userID, id, isAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to access this device"})
		return
	}

	target, err := findHost(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": strings.ReplaceAll(err.Error(), "\"", "'")})
		return
	}

	if err := SendWol(target); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "WoL Failed: " + strings.ReplaceAll(err.Error(), "\"", "'")})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Magic Packet sent successfully to " + target.Name})
}

func handleShutdown(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	if !isAuthorizedForDevice(userID, id, isAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to access this device"})
		return
	}

	target, err := findHost(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": strings.ReplaceAll(err.Error(), "\"", "'")})
		return
	}

	err = RemoteShutdown(target)

	c.JSON(http.StatusOK, gin.H{"message": "Shutdown command received from " + target.Name})
}

// ---- Admin API ----

func handleGetUsers(c *gin.Context) {
	var users []User
	// Preload the devices for the users so the admin can see them
	if err := DB.Preload("Devices").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	// We don't want to return password hashes, so we clear them manually or map to a DTO
	// However, json:"-" on PasswordHash already hides it.
	c.JSON(http.StatusOK, users)
}

type CreateUserRequest struct {
	Email    string   `json:"email" binding:"required"`
	Password string   `json:"password" binding:"required"`
	IsAdmin  bool     `json:"is_admin"`
	Devices  []string `json:"devices"`
}

func handleCreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	err = CreateUser(req.Email, hash, req.IsAdmin, req.Devices)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
}

func handleDeleteUser(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Prevent self-deletion if needed, but for simplicity we'll just delete
	if err := DB.Delete(&User{}, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func handleCheckSetup(c *gin.Context) {
	hasAdmins, err := HasAdmins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check setup status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"needs_setup": !hasAdmins})
}


func main() {
	fmt.Println("Main Starting...")

	// Parse CLI flags
	addUserEmail := flag.String("adduser", "", "Email of the user to add")
	addUserPass := flag.String("password", "", "Password for the new user")
	addUserAdmin := flag.Bool("admin", false, "Make the new user an admin")
	addUserDevices := flag.String("devices", "", "Comma-separated list of allowed device IDs")
	flag.Parse()

	// Initialize Database
	InitDB()

	// Handle CLI user creation
	if *addUserEmail != "" {
		if *addUserPass == "" {
			log.Fatal("Password is required when adding a user")
		}
		hash, err := HashPassword(*addUserPass)
		if err != nil {
			log.Fatalf("Error hashing password: %v", err)
		}
		devices := []string{}
		if *addUserDevices != "" {
			devices = strings.Split(*addUserDevices, ",")
		}
		err = CreateUser(*addUserEmail, hash, *addUserAdmin, devices)
		if err != nil {
			log.Fatalf("Error creating user: %v", err)
		}
		fmt.Printf("User %s created successfully.\n", *addUserEmail)
		os.Exit(0)
	}

	// Start the ping manager in the background
	go StartPingManager()

	var err error = nil
	//http server for API
	r := gin.Default()

	r.Use(cors.Default())

	// Public Routes
	r.POST("/api/login", handleLogin)
	r.GET("/api/setup", handleCheckSetup)

	// Protected Routes
	protected := r.Group("/api")
	protected.Use(AuthMiddleware())
	
	protected.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	protected.GET("/hosts", handleGetHosts)
	protected.POST("/wol/:id", handleWOL)
	protected.POST("/shutdown/:id", handleShutdown)
	
	// Admin Routes
	adminGroup := protected.Group("/users")
	adminGroup.Use(AdminMiddleware())
	adminGroup.GET("", handleGetUsers)
	adminGroup.POST("", handleCreateUser)
	adminGroup.DELETE("/:id", handleDeleteUser)

	// Serve static files from the React frontend "dist" folder
	if _, err := os.Stat("/app/frontend/dist/index.html"); err == nil {
		r.Static("/assets", "/app/frontend/dist/assets")
		r.StaticFile("/power.svg", "/app/frontend/dist/power.svg")
		r.LoadHTMLGlob("/app/frontend/dist/index.html")

		// Catch-all route for React Router (if you ever use it) or just to serve the main HTML page
		r.NoRoute(func(c *gin.Context) {
			c.HTML(http.StatusOK, "index.html", nil)
		})
	}

	// Get port from environment variable, default to 8080 if not set
	port := os.Getenv("PORT")
	if port == "" {
		port = "7500"
	}

	r.Run(":" + port)
	log.Fatal(err)
}
