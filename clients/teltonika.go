package clients

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/alekzander13/server_gps_service/models"
	"github.com/alekzander13/server_gps_service/utils"
)

type Teltonika models.ProtocolModel

func (T *Teltonika) GetBadPacketByte() []byte {
	return []byte{0}
}

func (T *Teltonika) ReturnError(err string) error {
	T.GPS.CountData = []byte{0}
	T.GPS.LastError = err
	return errors.New(T.GPS.LastError)
}

func (T *Teltonika) ParseData() error {
	T.GPS.LastConnect = time.Now().Local().Format("02.01.2006 15:04:05")
	T.GPS.LastInfo = ""
	T.GPS.LastError = "no data"

	if T.GPS.Name == "" {
		lenPack, err := strconv.ParseInt(hex.EncodeToString(T.Input[:2]), 16, 64)
		if err != nil {
			return T.ReturnError("error parse length packet " + err.Error())
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

	dataCRC := utils.CheckSumCRC16(T.Input)

	if origCRC != uint64(dataCRC) {
		return T.ReturnError(fmt.Sprintf("error crc sum: origCRC= %d, dataCRC= %d\n", origCRC, dataCRC))
	}

	CodecID := hex.EncodeToString([]byte{T.Input[0]})
	switch CodecID {
	case "08":
		T.GPS = parceGPSData8Codec(T.Input[1:], T.GPS, T.ChkPar, T.Path)
	case "8e":
		T.GPS = parceGPSData8ECodec(T.Input[1:], T.GPS, T.ChkPar, T.Path)
	default:
		return T.ReturnError("error codecID " + CodecID)
	}

	return nil
}

func parceGPSData8ECodec(input []byte, GPS models.GPSInfo, chkPar models.ChkParams, path string) models.GPSInfo {
	GPS.LastError = ""
	GPS.LastInfo = ""

	countData := int(input[0])
	GPS.CountData = []byte{0, 0, 0, byte(int8(countData))}

	posInInput := 1

	for i := 0; i < countData; i++ {
		GPS.LastError = ""
		var gpsData models.GPSData

		data := input[posInInput : posInInput+8]
		posInInput += 8

		encodedStr := hex.EncodeToString(data)
		intData, err := strconv.ParseInt(encodedStr, 16, 64)
		gpsData.DateTime = time.Date(2000, time.January, 01, 0, 0, 0, 0, time.UTC)
		if err == nil {
			gpsData.DateTime = time.Unix(intData/1000, 0).In(time.UTC)
		} else {
			GPS.LastError = "error parse time: " + err.Error()
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
			GPS.LastError = "error parse lng: " + err.Error()
		}

		//Lat
		data = input[posInInput : posInInput+4]
		posInInput += 4
		encodedStr = hex.EncodeToString(data)
		intData, err = strconv.ParseInt(encodedStr, 16, 32)
		if err == nil {
			gpsData.Lat = float64(intData) / 10000000.0
		} else {
			GPS.LastError = "error parse lat: " + err.Error()
		}

		//2b - Altitude In meters above sea level1
		data = input[posInInput : posInInput+2]
		posInInput += 2
		encodedStr = hex.EncodeToString(data)
		gpsData.Alt, err = strconv.ParseInt(encodedStr, 16, 32)
		if err != nil {
			GPS.LastError = "error parse altitude: " + err.Error()
		}

		//2b - Angle In degrees, 0 is north, increasing clock-wise 1
		data = input[posInInput : posInInput+2]
		posInInput += 2
		encodedStr = hex.EncodeToString(data)
		gpsData.Angle, err = strconv.ParseInt(encodedStr, 16, 32)
		if err != nil {
			GPS.LastError = "error parse angle: " + err.Error()
		}

		//1b - Satellites Number of visible satellites1
		gpsData.Sat = int64(input[posInInput])
		posInInput++

		//2b - Speed Speed in km/h. 0x0000 if GPS data is inval
		data = input[posInInput : posInInput+2]
		posInInput += 2
		encodedStr = hex.EncodeToString(data)
		gpsData.Speed, err = strconv.ParseInt(encodedStr, 16, 64)
		if err != nil {
			GPS.LastError = "error parse speed: " + err.Error()
		}

		//posInInput = 34
		//IO ELEMENT
		posInInput += 2 //0 – данные созданы не по событию

		//Общее кол-во передаваемых датчиков
		data = input[posInInput : posInInput+2]
		posInInput += 2
		countAllIO, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64)
		if err != nil {
			GPS.LastError = "error parse io element count: " + err.Error()
		} else {
			var c int64
			for c = 0; c < countAllIO; c++ {
				//0 - 1b, 1 - 2b, 2 - 4b, 3 - 8b, 4 - Xb
				switch c {
				case 0:
					data = input[posInInput : posInInput+2]
					posInInput += 2
					countIO, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64) // Кол-во датчиков разрядности 1 байт
					if err != nil {
						GPS.LastError = "error parse io element count 1b: " + err.Error()
					} else {
						var i int64
						for i = 0; i < countIO; i++ {
							data = input[posInInput : posInInput+2]
							posInInput += 2
							id, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64)
							if err != nil {
								GPS.LastError = "error parse io element id 1b: " + err.Error()
							} else {
								d := int(input[posInInput])
								posInInput++
								gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
							}
						}
					}

				case 1:
					data = input[posInInput : posInInput+2]
					posInInput += 2
					countIO, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64) // Кол-во датчиков разрядности 2 байта
					if err != nil {
						GPS.LastError = "error parse io element count 2b: " + err.Error()
					} else {
						var i int64
						for i = 0; i < countIO; i++ {
							data = input[posInInput : posInInput+2]
							posInInput += 2
							id, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64)
							if err != nil {
								GPS.LastError = "error parse io element id 2b: " + err.Error()
							} else {
								data = input[posInInput : posInInput+2]
								posInInput += 2
								d, err := strconv.ParseInt(hex.EncodeToString(data), 16, 16)
								if err == nil {
									switch id {
									case 66:
										gpsData.AccV = float64(d) / 1000
									case 67:
										gpsData.BatV = float64(d) / 1000
									default:
										gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
									}
								} else {
									GPS.LastError = "error parse io param 2b: " + err.Error()
								}
							}

						}
					}
				case 2:
					data = input[posInInput : posInInput+2]
					posInInput += 2
					countIO, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64) // Кол-во датчиков разрядности 4 байта
					if err != nil {
						GPS.LastError = "error parse io element count 4b: " + err.Error()
					} else {
						var i int64
						for i = 0; i < countIO; i++ {
							data = input[posInInput : posInInput+2]
							posInInput += 2
							id, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64)
							if err != nil {
								GPS.LastError = "error parse io element id 4b: " + err.Error()
							} else {
								data = input[posInInput : posInInput+4]
								posInInput += 4
								d, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64) //32
								if err == nil {
									gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
								} else {
									GPS.LastError = "error parse io param 4b: " + err.Error()
								}
							}
						}
					}
				case 3:
					data = input[posInInput : posInInput+2]
					posInInput += 2
					countIO, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64) // Кол-во датчиков разрядности 8 байт
					if err != nil {
						GPS.LastError = "error parse io element count 8b: " + err.Error()
					} else {
						var i int64
						for i = 0; i < countIO; i++ {
							data = input[posInInput : posInInput+2]
							posInInput += 2
							id, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64)
							if err != nil {
								GPS.LastError = "error parse io element id 8b: " + err.Error()
							} else {
								data = input[posInInput : posInInput+8]
								posInInput += 8
								d, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64)
								if err == nil {
									gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
								} else {
									GPS.LastError = "error parse io param 8b: " + err.Error()
								}
							}
						}
					}
				case 4:
					data = input[posInInput : posInInput+2]
					posInInput += 2
					countIO, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64) // Nx
					if err != nil {
						GPS.LastError = "error parse io element count NXb: " + err.Error()
					} else {
						var i int64
						for i = 0; i < countIO; i++ {
							data = input[posInInput : posInInput+2]
							posInInput += 2
							id, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64)
							if err != nil {
								GPS.LastError = "error parse io element id NXb: " + err.Error()
							} else {
								lenght, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64)
								if err != nil {
									GPS.LastError = "error parse len io param NXb: " + err.Error()
								} else {
									data = input[posInInput : posInInput+int(lenght)]
									posInInput += int(lenght)
									d, err := strconv.ParseInt(hex.EncodeToString(data), 16, 64)
									if err == nil {
										gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
									} else {
										GPS.LastError = "error parse io param NXb: " + err.Error()
									}
								}
							}
						}
					}
				default:

				}
			}

		}

		err = GPS.Chk(gpsData, chkPar)
		if err != nil {
			GPS.LastError = err.Error()
		} else {
			//GPS.LastError = ""
		}

		GPS.LastInfo = gpsData.DateTime.Format("02.01.06 ") + gpsData.ToString()

		if GPS.LastError != "" || err != nil {
			//save to error
			var errGPS models.GPSInfo
			errGPS = GPS
			errGPS.GpsD = gpsData
			if err := errGPS.SaveToError(path); err != nil {
				utils.ChkErrFatal(err)
			}
		} else {
			//save to file
			GPS.GpsD = gpsData
			if err := GPS.SaveToFile(path); err != nil {
				utils.ChkErrFatal(err)
			}
		}

	}

	return GPS
}

func parceGPSData8Codec(input []byte, GPS models.GPSInfo, chkPar models.ChkParams, path string) models.GPSInfo {
	GPS.LastError = ""
	GPS.LastInfo = ""

	countData := int(input[0])
	GPS.CountData = []byte{0, 0, 0, byte(int8(countData))}

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
			GPS.LastError = "error parse time: " + err.Error()
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
			GPS.LastError = "error parse lng: " + err.Error()
		}

		//Lat
		data = input[posInInput : posInInput+4]
		posInInput += 4
		encodedStr = hex.EncodeToString(data)
		intData, err = strconv.ParseInt(encodedStr, 16, 32)
		if err == nil {
			gpsData.Lat = float64(intData) / 10000000.0
		} else {
			GPS.LastError = "error parse lat: " + err.Error()
		}

		//2b - Altitude In meters above sea level1
		data = input[posInInput : posInInput+2]
		posInInput += 2
		encodedStr = hex.EncodeToString(data)
		gpsData.Alt, err = strconv.ParseInt(encodedStr, 16, 16)
		if err != nil {
			GPS.LastError = "error parse altitude: " + err.Error()
		}

		//2b - Angle In degrees, 0 is north, increasing clock-wise 1
		data = input[posInInput : posInInput+2]
		posInInput += 2
		encodedStr = hex.EncodeToString(data)
		gpsData.Angle, err = strconv.ParseInt(encodedStr, 16, 16)
		if err != nil {
			GPS.LastError = "error parse angle: " + err.Error()
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
			GPS.LastError = "error parse speed: " + err.Error()
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
						default:
							gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
						}
					} else {
						GPS.LastError = "error parse io param 2b: " + err.Error()
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
						gpsData.OtherID = append(gpsData.OtherID, fmt.Sprintf("id %d=%d;", id, d))
					} else {
						GPS.LastError = "error parse io param 4b: " + err.Error()
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
						GPS.LastError = "error parse io param 8b: " + err.Error()
					}
				}
			}
		}

		err = GPS.Chk(gpsData, chkPar)
		if err != nil {
			GPS.LastError = err.Error()
		} else {
			GPS.LastError = ""
		}

		GPS.LastInfo = gpsData.DateTime.Format("02.01.06 ") + gpsData.ToString()

		if GPS.LastError != "" || err != nil {
			//save to error
			var errGPS models.GPSInfo
			errGPS = GPS
			errGPS.GpsD = gpsData
			if err := errGPS.SaveToError(path); err != nil {
				utils.ChkErrFatal(err)
			}
		} else {
			//save to file
			GPS.GpsD = gpsData
			if err := GPS.SaveToFile(path); err != nil {
				utils.ChkErrFatal(err)
			}
		}

	}

	return GPS
}
