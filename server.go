package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"gps_clients/server_gps_service/clients"
	"gps_clients/server_gps_service/config"
	"gps_clients/server_gps_service/models"
	"gps_clients/server_gps_service/utils"
)

type SrvFuncer interface {
	ParseData() error
	GetBadPacketByte() []byte
}

func GetBadPacketByte(s SrvFuncer) []byte {
	return s.GetBadPacketByte()
}

func ParseGPSData(s SrvFuncer) error {
	return s.ParseData()
}

type Server struct {
	Addr         string
	IdleTimeout  time.Duration
	MaxReadBytes int64
	LastRequest  time.Time

	GPS map[string]models.GPSInfo

	listener   net.Listener
	conns      map[*conn]struct{}
	allcons    int
	mu         sync.Mutex
	inShutdown bool
}

func (srv *Server) ListenAndServe() error {
	if srv.Addr == "" {
		elog.Error(1, "empty port server")
		return errors.New("empty port server")
	}

	srv.GPS = make(map[string]models.GPSInfo)

	listen, err := net.Listen("tcp", ":"+srv.Addr)
	if err != nil {
		elog.Error(1, srv.Addr+": "+err.Error())
		return err
	}

	elog.Info(1, "tcp client start on "+srv.Addr)

	defer listen.Close()

	srv.listener = listen

	for {
		if srv.inShutdown {
			if len(srv.conns) == 0 {
				return nil
			}
			continue
		}

		newConn, err := listen.Accept()
		if err != nil {
			if srv.inShutdown {
				continue
			}
			elog.Error(1, srv.Addr+": "+err.Error())
			//AddToLog(GetProgramPath()+"-error.txt", fmt.Sprint(srv.inShutdown)+" - "+err.Error())
			continue
		}

		conn := &conn{
			Conn:          newConn,
			IdleTimeout:   srv.IdleTimeout,
			MaxReadBuffer: srv.MaxReadBytes,
		}

		srv.addConn(conn)
		srv.LastRequest = time.Now()
		conn.SetDeadline(time.Now().Add(conn.IdleTimeout))
		go srv.handle(conn)
	}
}

func (srv *Server) addConn(c *conn) {
	defer srv.mu.Unlock()
	srv.mu.Lock()
	if srv.conns == nil {
		srv.conns = make(map[*conn]struct{})
	}
	srv.conns[c] = struct{}{}
	srv.allcons++
}

func (srv *Server) deleteConn(conn *conn) {
	defer srv.mu.Unlock()
	srv.mu.Lock()
	delete(srv.conns, conn)
}

//CountLiveConn return count live connects
func (srv *Server) CountLiveConn() int {
	defer srv.mu.Unlock()
	srv.mu.Lock()
	return len(srv.conns)
}

//CountAllConn return count all connects
func (srv *Server) CountAllConn() int {
	defer srv.mu.Unlock()
	srv.mu.Lock()
	return srv.allcons
}

//Shutdown close server
func (srv *Server) Shutdown() {
	countForStop := 10
	srv.inShutdown = true
	elog.Info(1, srv.Addr+" is shutting down...")

	srv.listener.Close()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		<-ticker.C
		elog.Info(1, fmt.Sprintf("server %s waiting on %v connections", srv.Addr, len(srv.conns)))

		if len(srv.conns) == 0 {
			return
		}
		countForStop--
		if countForStop == 0 {
			elog.Info(1, srv.Addr+" Force close connections...")
			for c := range srv.conns {
				c.Close()
			}
			elog.Info(1, srv.Addr+" Force close connections completed")
		}
	}
}

func (srv *Server) SetGPS(gps models.GPSInfo) {
	defer srv.mu.Unlock()
	srv.mu.Lock()
	srv.GPS[gps.Name] = gps
}

func (srv *Server) GetGPS(name string) models.GPSInfo {
	defer srv.mu.Unlock()
	srv.mu.Lock()
	if _, ok := srv.GPS[name]; !ok {
		return srv.GPS[name]
	}
	return models.GPSInfo{}
}

func (srv *Server) GetGPSList() []models.GPSInfo {
	var gps []models.GPSInfo
	defer srv.mu.Unlock()
	srv.mu.Lock()
	for _, v := range srv.GPS {
		gps = append(gps, v)
	}
	return gps
}

func (srv *Server) handle(conn *conn) {
	defer func() {
		elog.Info(1, fmt.Sprintf("%s<-%s - connect close - %s",
			utils.GetPortAdr(conn.Conn.LocalAddr().String()),
			utils.GetPortAdr(conn.Conn.RemoteAddr().String()),
			time.Now().Local().Format("02.01.2006 15:04:05")))
		conn.Close()
		srv.deleteConn(conn)
	}()

	elog.Info(1, fmt.Sprintf("%s<-%s - new connect - %s",
		utils.GetPortAdr(conn.Conn.LocalAddr().String()),
		utils.GetPortAdr(conn.Conn.RemoteAddr().String()),
		time.Now().Local().Format("02.01.2006 15:04:05")))

	input := make([]byte, srv.MaxReadBytes)

	gps := &clients.Teltonika{}
	gps.ChkPar.Sat = config.Config.MinSatel
	gps.Path = config.Config.PathToSave

	for {
		reqlen, err := conn.Read(input)
		if err != nil {
			if err != io.EOF {
				elog.Error(1, err.Error())
			}
			return
		}

		if strings.HasPrefix(string(input[:reqlen]), "getinfo") {
			elog.Info(1, fmt.Sprintf("%s<-%s - get info - %s",
				utils.GetPortAdr(conn.Conn.LocalAddr().String()),
				utils.GetPortAdr(conn.Conn.RemoteAddr().String()),
				time.Now().Local().Format("02.01.2006 15:04:05")))
			var port models.PortInfo
			port.Name = srv.Addr
			port.Gps = srv.GetGPSList()
			body, err := json.Marshal(port)
			if err != nil {
				conn.Send([]byte(err.Error()))
			}
			conn.Send(body)
		} else {
			gps.Input = input[:reqlen]

			err = ParseGPSData(gps)

			if gps.GPS.Name != "" {
				srv.SetGPS(gps.GPS)
			}

			if err != nil {
				elog.Error(1, fmt.Sprintf("%s<-%s - GPS :%s - %s - %s",
					utils.GetPortAdr(conn.Conn.LocalAddr().String()),
					utils.GetPortAdr(conn.Conn.RemoteAddr().String()),
					gps.GPS.Name,
					err.Error(),
					time.Now().Local().Format("02.01.2006 15:04:05")))
				conn.Send(GetBadPacketByte(gps))
				continue
			}

			elog.Info(1, fmt.Sprintf("%s<-%s - GPS :%s - %s",
				utils.GetPortAdr(conn.Conn.LocalAddr().String()),
				utils.GetPortAdr(conn.Conn.RemoteAddr().String()),
				gps.GPS.Name,
				time.Now().Local().Format("02.01.2006 15:04:05")))

			conn.Send(gps.GPS.CountData)
			continue
		}

		conn.Send(GetBadPacketByte(gps))
	}
}
