package server

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kakeetopius/qosm/internal/db"
)

func (app *Server) GetInterfaceSettingsPopUp(c *gin.Context) {
	ifaceName := c.Param("ifaceName")

	iface, found := app.QoSManager.Ifaces[ifaceName]
	if !found {
		c.Error(fmt.Errorf("unknown interface: %s", ifaceName))
		return
	}

	c.HTML(http.StatusOK, "interface_settings", iface)
}

func (app *Server) PostInterfaceSettings(c *gin.Context) {
	ifaceName := c.Param("ifaceName")
	enableQoS := c.PostForm("qos_enabled") != ""
	disableQoS := !enableQoS
	rateStr := c.PostForm("rate")
	autorate := c.PostForm("auto_rate") != ""

	var err error
	defer func() {
		if err != nil {
			c.Error(err)
		}
	}()

	iface, found := app.QoSManager.Ifaces[ifaceName]
	if !found {
		err = fmt.Errorf("unknown interface: %s", ifaceName)
		return
	}

	if disableQoS && iface.QoSEnabled {
		err = app.QoSManager.DisableTcOnInterface(ifaceName)
		if err != nil {
			return
		}
		iface.QoSEnabled = false
		c.HTML(http.StatusOK, "interface_table_row", gin.H{
			"Iface":   iface,
			"Message": "Disabled QoS on interface: " + ifaceName,
		})
		return
	}

	rate, err := rateFromString(rateStr)
	if err != nil {
		c.Error(err)
		return
	}
	if autorate {
		rate = iface.LinkSpeed
	}
	if enableQoS && !iface.QoSEnabled {
		err = app.QoSManager.EnableTcOnInterface(ifaceName, uint32(rate))
		if err != nil {
			c.Error(err)
			return
		}
		iface.QoSEnabled = true
		c.HTML(http.StatusOK, "interface_table_row", gin.H{
			"Iface":   iface,
			"Message": "Successfully enabled QoS on interface: " + ifaceName,
		})
		return
	}

	if rate != iface.ShapingRate {
		err = app.QoSManager.ChangeInterfaceRate(ifaceName, rate)
		if err != nil {
			c.Error(err)
			return
		}
		c.HTML(http.StatusOK, "interface_table_row", gin.H{
			"Iface":   iface,
			"Message": "Successfully changed rate for interface " + ifaceName + " to " + rateStr + " Mbps",
		})
		return
	}
}

func (app *Server) PostDNSSettings(c *gin.Context) {
	primaryDNS := c.PostForm("primary_dns")
	dnsOverride := c.PostForm("dns_override") == "on"

	err := db.UpdateSettingsField(app.DB, "dns_override", dnsOverride)
	if err != nil {
		c.Error(err)
		return
	}
	app.Settings.DNSOverride = dnsOverride

	ip := net.ParseIP(primaryDNS)
	if ip == nil {
		err = fmt.Errorf("invalid primary dns: %v", primaryDNS)
		c.Error(err)
		return
	}

	err = db.UpdateSettingsField(app.DB, "primary_dns", primaryDNS)
	if err != nil {
		c.Error(err)
		return
	}
	app.Settings.PrimaryDNS = primaryDNS

	SendSuccessMessage(c)
}

func (app *Server) PostSecuritySettings(c *gin.Context) {
	var sessionTimeout int

	fmt.Sscanf(c.PostForm("session_timeout"), "%d", &sessionTimeout)
	err := db.UpdateSettingsField(app.DB, "session_timeout", sessionTimeout)
	if err != nil {
		c.Error(err)
		return
	}
	app.Settings.SessionTimeout = sessionTimeout

	SendSuccessMessage(c)
}

func SendSuccessMessage(c *gin.Context, message ...string) {
	var msg string
	if len(message) == 0 {
		msg = "Settings applied successfully ✔"
	} else {
		msg = message[0]
	}

	c.HTML(http.StatusOK, "toast_success", gin.H{
		"Message": msg,
	})
}

func rateFromString(rate string) (uint32, error) {
	if rate == "" {
		return 0, fmt.Errorf("please provide a rate for the interface")
	}

	rateInt, err := strconv.Atoi(rate)
	if err != nil {
		return 0, fmt.Errorf("invalid rate: %s", rate)
	}
	return uint32(rateInt), nil
}
