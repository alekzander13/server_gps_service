package clients

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"gps_clients/server_gps_service/hash"
	"gps_clients/server_gps_service/models"
	"gps_clients/server_gps_service/utils"
	"strconv"
	"time"
)

type Bitrek models.ProtocolModel

func (T *Bitrek) GetBadPacketByte() []byte {
	return []byte{0}
}

func (T *Bitrek) ReturnError(err string) error {
	T.GPS.CountData = []byte{0}
	T.GPS.LastError = err
	return errors.New(T.GPS.LastError)
}

func (T *Bitrek) ParseData() error {
	defer func() {
		if recMes := recover(); recMes != nil {
			utils.AddToLog(utils.GetProgramPath()+"-error.txt", recMes)
		}
	}()
	T.GPS.LastConnect = time.Now().Local().Format("02.01.2006 15:04:05")
	T.GPS.LastInfo = ""
	T.GPS.LastError = "no data"

	if T.GPS.Name == "" {
		lenPack, err := strconv.ParseInt(hex.EncodeToString(T.Input[:2]), 16, 64)
		if err != nil {
			return T.ReturnError("error parse length packet " + err.Error())
		}

		if int(lenPack+2) != len(T.Input) {
			return T.ReturnError(fmt.Sprintf("error length name: %d != %d", lenPack, len(T.Input)-2))
		}

		var i int64
		for i = 2; i < lenPack+2; i++ {
			T.GPS.Name += string(T.Input[i])
		}

		T.GPS.CountData = []byte{1}
		T.GPS.LastError = ""
		return nil
	}

	must := []byte{0, 0, 0, 0}
	have := make([]byte, 4)

	copy(have, T.Input)

	if !bytes.Equal(have, must) {
		//if HEADER not 0000 = send bad request
		return T.ReturnError("bad header " + string(have))
	}

	lenPacket, err := strconv.ParseInt(hex.EncodeToString(T.Input[4:8]), 16, 64)
	if err != nil {
		return T.ReturnError("error parse length packet " + err.Error())
	}

	T.Input = T.Input[8:]

	origByteCRC := T.Input[lenPacket:]

	T.Input = T.Input[:lenPacket]

	origCRC, err := strconv.ParseUint(hex.EncodeToString(origByteCRC), 16, 64)
	if err != nil {
		return T.ReturnError("error parse crc packet " + err.Error())
	}

	dataCRC := hash.CheckSumCRC16(T.Input)

	if origCRC != uint64(dataCRC) {
		return T.ReturnError(fmt.Sprintf("error crc sum: origCRC= %d, dataCRC= %d\n", origCRC, dataCRC))
	}

	CodecID := hex.EncodeToString([]byte{T.Input[0]})

	if CodecID != "08" {
		return T.ReturnError("bad codecID: " + CodecID)
	}

	T.ParceGPSData8Codec(T.Input[1:])

	return nil
}

func (T *Bitrek) ParceGPSData8Codec(input []byte) {
	T.GPS.LastError = ""
	T.GPS.LastInfo = ""

	countData := int(input[0])
	T.GPS.CountData = []byte{0, 0, 0, byte(int8(countData))}

	posInInput := 1

	for i := 0; i < countData; i++ {
		var gpsData models.GPSData

		data := input[posInInput : posInInput+8]
		posInInput += 8

		encodedStr := hex.EncodeToString(data)
		intData, err := strconv.ParseInt(encodedStr, 16, 64)
		gpsData.DateTime = time.Date(2000, time.January, 01, 0, 0, 0, 0, time.UTC)
		if err == nil {
			gpsData.DateTime = time.Unix(intData/1000, 0).In(time.UTC)
		} else {
			T.GPS.LastError = "error parse time: " + err.Error()
		}

		posInInput++ //Prioritet

		//Lng
		data = input[posInInput : posInInput+4]
		posInInput += 4
		encodedStr = hex.EncodeToString(data)
		intData, err = strconv.ParseInt(encodedStr, 16, 32)
		if err == nil {
			gpsData.Lng = float64(intData) / 10000000.0
		} else {
			T.GPS.LastError = "error parse lng: " + err.Error()
		}

		//Lat
		data = input[posInInput : posInInput+4]
		posInInput += 4
		encodedStr = hex.EncodeToString(data)
		intData, err = strconv.ParseInt(encodedStr, 16, 32)
		if err == nil {
			gpsData.Lat = float64(intData) / 10000000.0
		} else {
			T.GPS.LastError = "error parse lat: " + err.Error()
		}

		//2b - Altitude In meters above sea level1
		data = input[posInInput : posInInput+2]
		posInInput += 2
		encodedStr = hex.EncodeToString(data)
		gpsData.Alt, err = strconv.ParseInt(encodedStr, 16, 16)
		if err != nil {
			T.GPS.LastError = "error parse altitude: " + err.Error()
		}

		//2b - Angle In degrees, 0 is north, increasing clock-wise 1
		data = input[posInInput : posInInput+2]
		posInInput += 2
		encodedStr = hex.EncodeToString(data)
		gpsData.Angle, err = strconv.ParseInt(encodedStr, 16, 16)
		if err != nil {
			T.GPS.LastError = "error parse angle: " + err.Error()
		}

		//1b - Satellites Number of visible satellites1
		gpsData.Sat = int64(input[posInInput])
		posInInput++

		//2b - Speed Speed in km/h. 0x0000 if GPS data is inval
		data = input[posInInput : posInInput+2]
		posInInput += 2
		encodedStr = hex.EncodeToString(data)
		gpsData.Speed, err = strconv.ParseInt(encodedStr, 16, 16)
		if err != nil {
			T.GPS.LastError = "error parse speed: " + err.Error()
		}

		//posInInput = 34
		//IO ELEMENT
		posInInput++ //0 – данные созданы не по событию

		countAllIO := int(input[posInInput])
		posInInput++ //Общее кол-во передаваемых датчиков

		for i := 0; i < countAllIO; i++ {
			switch i {
			case 0:
				countIO := int(input[posInInput]) // Кол-во датчиков разрядности 1 байт
				posInInput++
				for i := 0; i < countIO; i++ {
					id := int(input[posInInput])
					posInInput++
					d := int(input[posInInput])
					posInInput++
					gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
				}
			case 1:
				countIO := int(input[posInInput]) // Кол-во датчиков разрядности 2 байта
				posInInput++
				for i := 0; i < countIO; i++ {
					id := int(input[posInInput])
					posInInput++
					data = input[posInInput : posInInput+2]
					posInInput += 2
					encodedStr = hex.EncodeToString(data)
					d, err := strconv.ParseInt(encodedStr, 16, 16)
					if err == nil {
						switch id {
						case 66:
							gpsData.AccV = float64(d) / 1000
						case 67:
							gpsData.BatV = float64(d) / 1000
						case 158:
							gpsData.Dut2 = int64(float32(d) * 0.1)
							gpsData.UseDut = true
						case 100:
							gpsData.Dut1 = d
							gpsData.UseDut = true
						case 159:
							gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("Tahometer=%.f;", float32(d)*0.25))
						case 9:
							gpsData.TempC = (float64(d) / 9.6) - 273
							gpsData.UseTempC = true
						default:
							gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
						}
					} else {
						T.GPS.LastError = "error parse io param 2b: " + err.Error()
					}
				}
			case 2:
				countIO := int(input[posInInput]) // Кол-во датчиков разрядности 4 байта
				posInInput++
				for i := 0; i < countIO; i++ {
					id := int(input[posInInput])
					posInInput++
					data = input[posInInput : posInInput+4]
					posInInput += 4
					encodedStr = hex.EncodeToString(data)
					d, err := strconv.ParseInt(encodedStr, 16, 64) //32
					if err == nil {
						switch id {
						case 153:
							gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("Odometer=%.0f;", float64(d)*0.005))
						default:
							gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
						}
					} else {
						T.GPS.LastError = "error parse io param 4b: " + err.Error()
					}
				}
			case 3:
				countIO := int(input[posInInput]) // Кол-во датчиков разрядности 8 байт
				posInInput++
				for i := 0; i < countIO; i++ {
					id := int(input[posInInput])
					posInInput++
					data = input[posInInput : posInInput+8]
					posInInput += 8
					encodedStr = hex.EncodeToString(data)
					d, err := strconv.ParseInt(encodedStr, 16, 64)
					if err == nil {
						gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
					} else {
						T.GPS.LastError = "error parse io param 8b: " + err.Error()
					}
				}
			}
		}

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
