package clients

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"gps_clients/server_gps_service/models"
	"gps_clients/server_gps_service/utils"
	"log"
	"strconv"
	"strings"
	"time"
)

type Wialon models.ProtocolModel

func (T *Wialon) GetBadPacketByte() []byte {
	return []byte{0}
}

func (T *Wialon) ReturnError(err string) error {
	T.GPS.CountData = []byte{0}
	T.GPS.LastError = err
	return errors.New(T.GPS.LastError)
}

func (T *Wialon) ParseData() error {
	defer func() {
		if recMes := recover(); recMes != nil {
			utils.AddToLog(utils.GetProgramPath()+"-error.txt", recMes)
		}
	}()
	T.GPS.LastConnect = time.Now().Local().Format("02.01.2006 15:04:05")
	T.GPS.LastInfo = ""
	T.GPS.LastError = "no data"

	if strings.HasPrefix(string(T.Input), "#") {
		T.WialonIPS()
	} else {
		T.WialonRetranslator_v1()
	}

	return nil
}

func (T *Wialon) WialonIPS() {
	body := T.Input

	bodySlice := strings.Split(string(body), "\r\n")
	if len(bodySlice) < 2 {
		T.GPS.LastError = "wrong input data"
		return
	}

	for _, v := range bodySlice {
		if v == "" {
			continue
		}

		slice := strings.Split(v, "#")
		if len(slice) < 3 {
			T.GPS.LastError = "wrong split data wialon ips: " + v
			return
		}

		switch slice[1] {
		case "L":
			s := strings.Split(slice[2], ";")
			T.GPS.Name = s[0]
		case "D", "SD":
			s := strings.Split(slice[2], ";")

			var gpsData models.GPSData
			var err error

			gpsData.DateTime, err = time.Parse("020106 150405", s[0]+" "+s[1])
			if err != nil {
				T.GPS.LastError = "error parce data: " + err.Error()
				return
			}

			gpsData.Lat = utils.ConvertCoordToFloat(s[2])
			gpsData.Lng = utils.ConvertCoordToFloat(s[4])
			gpsData.Sat, _ = strconv.ParseInt(s[9], 10, 64)
			gpsData.Alt, _ = strconv.ParseInt(s[8], 10, 64)
			gpsData.Speed, _ = strconv.ParseInt(s[6], 10, 64)
			gpsData.Angle, _ = strconv.ParseInt(s[7], 10, 64)

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
				if err := errGPS.SaveToError(T.Path); err != nil {
					utils.ChkErrFatal(err)
				}
			} else {
				//save to file
				T.GPS.GpsD = gpsData
				if err := T.GPS.SaveToFile(T.Path); err != nil {
					utils.ChkErrFatal(err)
				}
			}
		}
	}
}

func (T *Wialon) WialonRetranslator_v1() {
	body := T.Input

	bodySlice := strings.Split(string(body), "0BBB")
	if len(bodySlice) < 2 {
		T.GPS.LastError = "wrong length data wialon retranslator"
		return
	}

	var gpsData models.GPSData

	for i, v := range bodySlice {
		if i == 0 {
			var err error
			var buf []byte
			lastIndex := 8

			tempBody := []byte(v)

			namePos := strings.Index(string(tempBody[lastIndex:]), "00")
			for i := lastIndex; i < namePos+lastIndex; i += 2 {
				buf = append(buf, tempBody[i], tempBody[i+1])
			}

			if r, err := hex.DecodeString(string(buf)); err == nil {
				T.GPS.Name = string(r)
			} else {
				T.GPS.LastError = "error gps name wialon retranslator"
				return
			}

			lastIndex += namePos + 2
			buf = tempBody[lastIndex : lastIndex+8]
			intData, err := strconv.ParseInt(string(buf), 16, 64)
			if err != nil {
				T.GPS.LastError = "error parce time wialon retranslator"
				return
			}

			gpsData.DateTime = time.Unix(intData, 0)
		} else {
			blockInfo, lastIndex, err := parseBlock([]byte(v))
			if err != nil {
				T.GPS.LastError = "error parce block info wialon retranslator"
				return
			}

			if blockInfo.Name == "posinfo" {
				posInfo, err := parsePosInfo([]byte(v), lastIndex)
				if err != nil {
					panic(err)
				}

				gpsData.Lat = posInfo.Lat
				gpsData.Lng = posInfo.Lng
				gpsData.Sat = posInfo.Sat
				gpsData.Speed = posInfo.Speed
				gpsData.Alt = int64(posInfo.High)
				gpsData.Angle = posInfo.Course

			} else {
				switch blockInfo.Type {
				case 4:
					res := []byte(v)
					res = res[lastIndex:]
					buf16Byte := make([]byte, 16)
					_, err = hex.Decode(buf16Byte, res)
					if err != nil {
						log.Println(err)
						continue
					}

					var double float64
					if err := binary.Read(bytes.NewBuffer(buf16Byte), binary.LittleEndian, &double); err != nil {
						log.Println(err)
						continue
					}
					gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("%s=%.2f;", blockInfo.Name, double))

				case 3:
					res := []byte(v)
					res = res[lastIndex:]
					buf16Byte := make([]byte, 16)
					_, err = hex.Decode(buf16Byte, res)
					if err != nil {
						log.Println(err)
						continue
					}

					var integer int
					if integer, err = strconv.Atoi(string(res)); err != nil {
						log.Println(err)
						continue
					}
					gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("%s=%d;", blockInfo.Name, integer))
				}
			}

		}
	}

	err := T.GPS.Chk(gpsData, T.ChkPar)
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
		if err := errGPS.SaveToError(T.Path); err != nil {
			utils.ChkErrFatal(err)
		}
	} else {
		//save to file
		T.GPS.GpsD = gpsData
		if err := T.GPS.SaveToFile(T.Path); err != nil {
			utils.ChkErrFatal(err)
		}
	}
}

type BlockInfo struct {
	Size int64
	Hide int64
	Type int64
	Name string
}

func parseBlock(body []byte) (BlockInfo, int, error) {
	var result BlockInfo
	var err error
	lastIndex := 0

	res := body[lastIndex : lastIndex+8]
	result.Size, err = strconv.ParseInt(string(res), 16, 64)
	if err != nil {
		return BlockInfo{}, -1, err
	} else {
		result.Size *= 2
	}
	lastIndex += 8

	res = body[lastIndex : lastIndex+2]
	result.Hide, err = strconv.ParseInt(string(res), 16, 64)
	if err != nil {
		return BlockInfo{}, -1, err
	}
	lastIndex += 2

	res = body[lastIndex : lastIndex+2]
	result.Type, err = strconv.ParseInt(string(res), 16, 64)
	if err != nil {
		return BlockInfo{}, -1, err
	}
	lastIndex += 2

	namePos := strings.Index(string(body[lastIndex:]), "00")
	res = []byte{}
	for i := lastIndex; i < namePos+lastIndex; i += 2 {
		res = append(res, body[i], body[i+1])
	}

	if r, e := hex.DecodeString(string(res)); e == nil {
		result.Name = string(r)
	} else {
		return BlockInfo{}, -1, e
	}
	lastIndex += namePos + 2

	return result, lastIndex, nil
}

type PosInfo struct {
	Lat    float64
	Lng    float64
	High   float64
	Speed  int64
	Course int64
	Sat    int64
}

func parsePosInfo(blockData []byte, lastIndexBlock int) (PosInfo, error) {
	var result PosInfo
	res := blockData[lastIndexBlock : lastIndexBlock+16]

	buf16Byte := make([]byte, 16)
	_, err := hex.Decode(buf16Byte, res)
	if err != nil {
		return PosInfo{}, err
	}

	if err := binary.Read(bytes.NewBuffer(buf16Byte), binary.LittleEndian, &result.Lng); err != nil {
		return PosInfo{}, err
	}

	lastIndexBlock += 16
	res = blockData[lastIndexBlock : lastIndexBlock+16]

	_, err = hex.Decode(buf16Byte, res)
	if err != nil {
		return PosInfo{}, err
	}

	if err := binary.Read(bytes.NewBuffer(buf16Byte), binary.LittleEndian, &result.Lat); err != nil {
		return PosInfo{}, err
	}

	lastIndexBlock += 16
	res = blockData[lastIndexBlock : lastIndexBlock+16]

	_, err = hex.Decode(buf16Byte, res)
	if err != nil {
		return PosInfo{}, err
	}

	if err := binary.Read(bytes.NewBuffer(buf16Byte), binary.LittleEndian, &result.High); err != nil {
		return PosInfo{}, err
	}

	lastIndexBlock += 16
	res = blockData[lastIndexBlock : lastIndexBlock+4]

	result.Speed, err = strconv.ParseInt(string(res), 16, 64)
	if err != nil {
		return PosInfo{}, err
	}

	lastIndexBlock += 4

	res = blockData[lastIndexBlock : lastIndexBlock+4]

	result.Course, err = strconv.ParseInt(string(res), 16, 64)
	if err != nil {
		return PosInfo{}, err
	}

	lastIndexBlock += 4

	res = blockData[lastIndexBlock:]

	result.Sat, err = strconv.ParseInt(string(res), 16, 64)
	if err != nil {
		return PosInfo{}, err
	}

	return result, nil
}
