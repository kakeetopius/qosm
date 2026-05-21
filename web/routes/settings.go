package routes

import (
	"fmt"
	"net"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/kakeetopius/qosm/internal/core/tc"
	"github.com/kakeetopius/qosm/web/db"
)

func (app *ServerCtx) SaveSystemSettings(c *gin.Context) {
	loggingLevel := c.PostForm("logging_level")
	var maxBandwidth int
	fmt.Sscanf(c.PostForm("max_bandwidth"), "%d", &maxBandwidth)

	err := db.UpdateSettingField(app.DB, "logging_level", loggingLevel)
	if err != nil {
		app.Logger.Error(err.Error())
	}
	app.Settings.LoggingLevel = loggingLevel

	err = db.UpdateSettingField(app.DB, "max_bandwidth", maxBandwidth)
	if err != nil {
		app.Logger.Error(err.Error())
	}
	app.Settings.MaxBandwidth = maxBandwidth

	c.Status(http.StatusOK)
}

func (app *ServerCtx) SaveInterfaceSettings(c *gin.Context) {
	ifaceNames := c.PostFormArray("interfaces")

	var err error
	for _, iface := range app.Ifaces {
		if slices.Contains(ifaceNames, iface.Name) {
			err = enableQoS(app, iface.Name)
		} else {
			err = disableQoS(app, iface.Name)
		}
		if err != nil {
			app.Logger.Error(err.Error())
		}
	}

	c.Status(http.StatusOK)
}

func (app *ServerCtx) SaveDNSSettings(c *gin.Context) {
	primaryDNS := c.PostForm("primary_dns")
	dnsOverride := c.PostForm("dns_override") == "on"

	err := db.UpdateSettingField(app.DB, "dns_override", dnsOverride)
	if err != nil {
		app.Logger.Error(err.Error())
	}
	app.Settings.DNSOverride = dnsOverride

	err = db.UpdateSettingField(app.DB, "primary_dns", primaryDNS)
	if err != nil {
		app.Logger.Error(err.Error())
	}
	app.Settings.PrimaryDNS = primaryDNS

	c.Status(http.StatusOK)
}

func (app *ServerCtx) SaveSecuritySettings(c *gin.Context) {
	var sessionTimeout int

	fmt.Sscanf(c.PostForm("session_timeout"), "%d", &sessionTimeout)
	err := db.UpdateSettingField(app.DB, "session_timeout", sessionTimeout)
	if err != nil {
		app.Logger.Error(err.Error())
	}
	app.Settings.SessionTimeout = sessionTimeout

	c.Status(http.StatusOK)
}

func enableQoS(app *ServerCtx, ifaceName string) error {
	iface := app.Ifaces[ifaceName]
	if iface.Enabled {
		return nil
	}
	htbCtx, err := tc.NewHTBCtx(ifaceName)
	if err != nil {
		return err
	}
	iface.Enabled = true
	iface.HTBCtx = htbCtx
	app.Ifaces[ifaceName] = iface

	return nil
}

func disableQoS(app *ServerCtx, ifaceName string) error {
	iface := app.Ifaces[ifaceName]
	if !iface.Enabled {
		return nil
	}
	if iface.HTBCtx != nil {
		err := iface.HTBCtx.FlushQdisc()
		if err != nil {
			return err
		}
	}

	iface.Enabled = false
	app.Ifaces[ifaceName] = iface
	return nil
}

func GetInterfaces() (map[string]Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ifaceMap := make(map[string]Interface)

	for _, iface := range ifaces {
		exists, err := tc.HasHTBQdisc(&iface)
		if err != nil {
			return nil, err
		}
		qosIface := Interface{
			Name:    iface.Name,
			Enabled: exists,
		}
		if exists {
			htb, err := tc.NewHTBCtx(iface.Name)
			if err != nil {
				return nil, err
			}
			qosIface.HTBCtx = htb
		}
		ifaceMap[iface.Name] = qosIface
	}

	return ifaceMap, nil
}
