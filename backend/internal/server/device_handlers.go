package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"secure-switch-backend/internal/config"
)

// Check if user is authorized for a specific device based on UserDevice mapping
func (a *App) isAuthorizedForDevice(userID uint, deviceID string, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	user, err := a.Store.GetUserByID(userID)
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

func (a *App) handleGetHosts(c *gin.Context) {
	hosts, err := a.Config.LoadHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error loading hosts"})
		return
	}

	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	// Filter hosts based on authorization
	var authorizedHosts []Host
	for i := range hosts {
		if a.isAuthorizedForDevice(userID, hosts[i].ID, isAdmin) {
			// Attach online state to hosts from cache
			state := a.Monitor.State(hosts[i].ID)
			hosts[i].Online = state.Online
			if state.LastPinged == "" {
				hosts[i].LastPinged = "N/A"
			} else {
				hosts[i].LastPinged = state.LastPinged
			}

			authorizedHosts = append(authorizedHosts, hosts[i])
		}
	}

	c.JSON(http.StatusOK, authorizedHosts)
}

func (a *App) authorizedTarget(c *gin.Context, lookupAction string) (*config.Host, bool) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	if !a.isAuthorizedForDevice(userID, id, isAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to access this device"})
		return nil, false
	}

	target, err := a.findHost(id)
	if err != nil {
		respondActionFailure(c, lookupAction, http.StatusNotFound, "Device not found")
		return nil, false
	}
	return target, true
}

func (a *App) handleWOL(c *gin.Context) {
	target, ok := a.authorizedTarget(c, "wol target lookup")
	if !ok {
		return
	}

	if err := a.Wake(target); err != nil {
		respondActionFailure(c, "wake-on-LAN", http.StatusInternalServerError, "Unable to send Wake-on-LAN packet")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Magic Packet sent successfully to " + target.Name})
}

func (a *App) handleShutdown(c *gin.Context) {
	target, ok := a.authorizedTarget(c, "shutdown target lookup")
	if !ok {
		return
	}

	err := a.Shutdown(target)
	if err != nil {
		respondActionFailure(c, "shutdown", http.StatusInternalServerError, "Unable to shut down device")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Shutdown command received from " + target.Name})
}
