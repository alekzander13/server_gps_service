package clients

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gps_clients/server_gps_service/models"
	"gps_clients/server_gps_service/utils"
)

type GryphonM01 models.ProtocolModel

func (T *GryphonM01) GetBadPacketByte() []byte {
	return []byte(string("ok;"))
}

func (T *GryphonM01) ReturnError(err string) error {
	T.GPS.LastError = err
	return errors.New(T.GPS.LastError)
}

func (T *GryphonM01) ParseData() error {
	defer func() {
		if recMes := recover(); recMes != nil {
			utils.AddToLog(utils.GetProgramPath()+"-error.txt", recMes)
		}
	}()
	T.GPS.LastConnect = time.Now().Local().Format("02.01.2006 15:04:05")
	T.GPS.LastInfo = ""
	T.GPS.LastError = "no data"

	dataMap, err := T.ParceForm()
	if err != nil {
		return T.ReturnError(err.Error())
	}

	v, ok := dataMap["a"]
	if !ok {
		return T.ReturnError("error: miss name key")
	}

	T.GPS.Name = v

	v, ok = dataMap["d"]
	if ok {
		return T.ParceGPSData(v)
	}

	return nil
}

func (T *GryphonM01) ParceForm() (form map[string]string, err error) {
	defer func() {
		if errMsg := recover(); errMsg != nil {
			err = fmt.Errorf("panic parce data: %v", errMsg)
		}
	}()

	form = make(map[string]string)

	ss := strings.Split(strings.TrimSpace(string(T.Input)), "&")
	//0 - GET http://77.91.169.124/dt.php?s=3
	for _, v := range ss[1:] {
		s := strings.Split(v, "=")
		form[s[0]] = s[1]
	}
	return
}

func (T *GryphonM01) ParceGPSData(info string) error {
	T.GPS.LastInfo = ""
	T.GPS.CountData = []byte("ok;")

	mapToSave := make(map[string][]models.GPSData)
	var listError []models.GPSInfo

	for _, s := range strings.Split(info, "_") {
		T.GPS.LastError = ""
		//260711,114432,5026.50150,3038.7875,1434,34.11,0,106.3,116.52,6,0
		v := strings.Split(s, ",")
		if len(v) < 10 {
			return T.ReturnError("bad lenght")
		}

		var gpsData models.GPSData
		var err error

		gpsData.DateTime, err = time.Parse("020106 150405", v[0]+" "+v[1])
		if err != nil {
			T.GPS.LastError = "error parse time: " + err.Error()
		} else {
			if gpsData.DateTime.Year() < 2015 {
				gpsData.DateTime = gpsData.DateTime.AddDate(0, 0, 7168)
			}
		}

		gpsData.Lat = utils.ConvertCoordToFloat(v[2])
		gpsData.Lng = utils.ConvertCoordToFloat(v[3])

		gpsData.AccV, _ = strconv.ParseFloat(v[4], 64)
		gpsData.AccV /= 100

		val, _ := strconv.ParseFloat(v[5], 64)
		val *= 1.852 //mile\h to k\h
		gpsData.Speed = int64(val)

		datchikiNum, _ := strconv.ParseInt(v[6], 10, 64)
		datchikiStr := strconv.FormatInt(datchikiNum, 2)
		for pos, r := range datchikiStr {
			switch pos {
			case 0:
				gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("Zajig=%s;", string(r)))
			case 1:
				gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("Acsel=%s;", string(r)))
			case 2:
				gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("Datchik2=%s;", string(r)))
			case 3:
				gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("Datchik1=%s;", string(r)))
			}
		}

		val, _ = strconv.ParseFloat(v[7], 64)
		gpsData.Alt = int64(val)

		val, _ = strconv.ParseFloat(v[8], 64)
		gpsData.Angle = int64(val)

		gpsData.Sat, _ = strconv.ParseInt(v[9], 10, 64)

		err = T.GPS.Chk(gpsData, T.ChkPar)
		if err != nil {
			T.GPS.LastError = err.Error()
		} else {
			T.GPS.LastError = ""
		}

		T.GPS.LastInfo = gpsData.DateTime.Format("02.01.06 ") + gpsData.ToString()

		if T.GPS.LastError != "" || err != nil {
			//save to error
			var errGPS models.GPSInfo
			errGPS = T.GPS
			errGPS.GpsD = gpsData
			listError = append(listError, errGPS)
		} else {
			mapToSave[gpsData.DateTime.Format("020106")] = append(mapToSave[gpsData.DateTime.Format("020106")], gpsData)
			T.GPS.GpsD = gpsData
		}
	}

	if err := T.GPS.SaveErrorList(T.Path, listError); err != nil {
		return err
	}

	if err := T.GPS.SaveToFileList(T.Path, mapToSave); err != nil {
		return err
	}

	return nil
}
