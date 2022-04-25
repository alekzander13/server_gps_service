package config

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"gps_clients/server_gps_service/utils"
)

var Config Configuration

type Configuration struct {
	ServiceName string   `json:"serivceName"`
	DescService string   `json:"descService"`
	Ports       []string `json:"ports"`
	PathToSave  string   `json:"pathToSave"`
	MinSatel    int64    `json:"minSatel"`
}

func setstandartconfig() {
	Config.ServiceName = "go_server_teltonika"
	Config.DescService = "TLKA gps-server service"
	Config.Ports = []string{"10000", "10001"}
	Config.PathToSave = "D:/UPC"
	Config.MinSatel = 4
}

func ReadConfig(fileName string) error {
	ok, err := utils.Exists(fileName)
	if err != nil {
		return err
	}
	if ok {
		configFile, err := ioutil.ReadFile(fileName)
		if err != nil {
			log.Print("Unable to read config file, switching to flag mode")
			return err
		}
		Config, err = unmarshalconfig(configFile)
		if err != nil {
			return err
		}
	} else {
		setstandartconfig()
		return writeconfigtofile(fileName)
	}

	return nil
}

//writeconfigtofile сохраняет конфигурацию настроек в файл
func writeconfigtofile(namefile string) error {
	body, err := Config.Marshal()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(namefile, body, 0777)
}

//unmarshalconfig разбор параметров
func unmarshalconfig(data []byte) (Configuration, error) {
	var r Configuration
	err := json.Unmarshal(data, &r)
	return r, err
}

//Marshal сбор параметров
func (r *Configuration) Marshal() ([]byte, error) {
	return json.Marshal(r)
}
