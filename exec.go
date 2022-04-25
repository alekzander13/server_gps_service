package main

import (
	"time"

	"github.com/alekzander13/include/gps_clients/config"
	"github.com/alekzander13/include/utils"
)

var servers map[string]*Server

func initServer() {
	servers = make(map[string]*Server)

	ports, err := utils.MakePortsFromSlice(config.Config.Ports)
	utils.ChkErrFatal(err)

	for _, p := range ports {
		srv := Server{
			Addr:         p,
			IdleTimeout:  180 * time.Second,
			MaxReadBytes: 10240, //2048
		}

		go srv.ListenAndServe()
		servers[p] = &srv
	}
}

func stopServers() {
	for _, s := range servers {
		s.Shutdown()
	}
}

func startServers() {
	for _, s := range servers {
		s.inShutdown = false
		go s.ListenAndServe()
	}
}
