package models

import (
	"errors"
	"fmt"
	"gps_clients/server_gps_service/utils"
	"os"
	"strings"
	"time"
)

type ServerInfo struct {
	Name  string     `json:"name"`
	Ports []PortInfo `json:"ports"`
}

type PortInfo struct {
	Name string    `json:"name"`
	Gps  []GPSInfo `json:"gps"`
}

type GPSInfo struct {
	Name        string  `json:"name"`
	LastConnect string  `json:"lastconnect"`
	LastInfo    string  `json:"lastinfo"`
	LastError   string  `json:"lasterror"`
	CountData   []byte  `json:"-"`
	GpsD        GPSData `json:"-"`
}

func (g *GPSInfo) SaveODPList(path string, sl []string) error {
	if len(sl) < 1 {
		return nil
	}

	if path == "" {
		path = utils.GetPathWhereExe()
	}
	path += "/ODP/"

	if err := os.MkdirAll(path, 0777); err != nil {
		return err
	}

	path += g.Name + ".txt"

	strToSave := ""

	for _, v := range sl {
		strToSave += fmt.Sprintf("%s\r\n", v)
	}

	if file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777); err != nil {
		return err
	} else {
		defer file.Close()
		_, err := file.WriteString(strToSave)
		return err
	}
}

func (g *GPSInfo) SaveToError(path string) error {
	if path == "" {
		path = utils.GetPathWhereExe()
	}
	path += "/Error/"

	if err := os.MkdirAll(path, 0777); err != nil {
		return err
	}

	path += g.Name + ".txt"

	if file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777); err != nil {
		return err
	} else {
		defer file.Close()
		_, err := file.WriteString("- " + g.LastError + "\r\n" +
			time.Now().Local().Format("02.01.2006 15:04:05") + "\r\n" +
			g.GpsD.DateTime.Format("02.01.06 ") + g.GpsD.ToString())
		return err
	}
}

func (g *GPSInfo) SaveErrorList(path string, sl []GPSInfo) error {
	if len(sl) < 1 {
		return nil
	}

	if path == "" {
		path = utils.GetPathWhereExe()
	}
	path += "/Error/"

	if err := os.MkdirAll(path, 0777); err != nil {
		return err
	}

	path += g.Name + ".txt"

	strToSave := ""

	for _, v := range sl {
		strToSave += fmt.Sprintf("-%s \r\n%s\r\n%s %s\r\n",
			v.LastError,
			time.Now().Local().Format("02.01.2006 15:04:05"),
			v.GpsD.DateTime.Format("02.01.06"),
			v.GpsD.ToString())
	}

	if file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777); err != nil {
		return err
	} else {
		defer file.Close()
		_, err := file.WriteString(strToSave)
		return err
	}
}

func (g *GPSInfo) SaveToFile(path string) error {
	if path == "" {
		path = utils.GetPathWhereExe()
	}
	path += g.GpsD.DateTime.Format("/06/01/02/")

	if err := os.MkdirAll(path, 0777); err != nil {
		return err
	}

	path += g.Name + ".txt"

	if file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777); err != nil {
		return err
	} else {
		defer file.Close()
		_, err := file.WriteString(g.GpsD.ToString())
		return err
	}
}

func (g *GPSInfo) SaveToFileList(path string, info map[string][]GPSData) error {
	if len(info) < 1 {
		return nil
	}

	if path == "" {
		path = utils.GetPathWhereExe()
	}

	for d, v := range info {
		if len(v) < 1 {
			continue
		}
		p, err := time.Parse("020106", d)
		if err != nil {
			return err
		}

		path += p.Format("/06/01/02/")

		if err := os.MkdirAll(path, 0777); err != nil {
			return err
		}

		path += g.Name + ".txt"

		strToSave := ""
		for _, s := range v {
			strToSave += fmt.Sprintf("%s", s.ToString())
		}
		if strToSave != "" {
			if file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777); err != nil {
				return err
			} else {
				defer file.Close()
				_, err := file.WriteString(strToSave)
				return err
			}
		}
	}

	return nil
}

func (g *GPSInfo) Chk(d GPSData, c ChkParams) error {
	if d.DateTime.Before(g.GpsD.DateTime) {
		return errors.New("Последнее время меньше предидущего")
	}

	if d.DateTime.After(time.Now().AddDate(0, 0, 1)) {
		return errors.New("Последнее время больше завтра")
	}

	if d.Sat < c.Sat {
		return fmt.Errorf("Спутников менее %d", c.Sat)
	}

	return nil
}

type GPSData struct {
	DateTime time.Time
	Lat      float64
	Lng      float64
	Alt      int64
	Angle    int64
	Sat      int64
	Speed    int64
	AccV     float64
	BatV     float64
	TempC    float64
	Dut1     int64
	Dut2     int64
	OtherID  []string
	UseDut   bool
	UseTempC bool
}

func (g *GPSData) ToString() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s;", g.DateTime.Format("150405"))
	fmt.Fprintf(&sb, "%f;", g.Lat)
	fmt.Fprintf(&sb, "%f;", g.Lng)
	fmt.Fprintf(&sb, "Altitude=%d;", g.Alt)
	fmt.Fprintf(&sb, "Angle=%d;", g.Angle)
	fmt.Fprintf(&sb, "SatCount=%d;", g.Sat)
	fmt.Fprintf(&sb, "Speed=%d;", g.Speed)
	fmt.Fprintf(&sb, "AccV=%.2f;", g.AccV)
	fmt.Fprintf(&sb, "BatV=%.2f;", g.BatV)
	if g.UseTempC {
		fmt.Fprintf(&sb, "TempC=%.1f;", g.TempC)
	}
	if g.UseDut {
		fmt.Fprintf(&sb, "Dut1=%d;Dut2=%d;Dut3=0;Dut4=0;", g.Dut1, g.Dut2)
	}
	for _, v := range g.OtherID {
		fmt.Fprintf(&sb, "%s", v)
	}
	sb.WriteString("\r\n")

	return sb.String()
}

type ChkParams struct {
	Sat int64
}

type ProtocolModel struct {
	Input  []byte
	Path   string
	ChkPar ChkParams
	GPS    GPSInfo
}
