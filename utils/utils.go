package utils

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func ChkErrFatal(err error) {
	if err != nil {
		AddToLog(GetProgramPath()+"-error.txt", err)
		os.Exit(1)
	}
}

//Exists exist file or folder
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func AddToLog(name string, info interface{}) {
	f, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		return
	}
	defer f.Close()

	date := time.Now().Local().Format("02.01.06 15:04:05 ")

	fmt.Fprintln(f, date, info)
}

func GetPathWhereExe() string {
	p, err := filepath.Abs(os.Args[0])
	if err != nil {
		log.Fatal(err)
	}

	return filepath.Dir(p)
}

func GetProgramPath() string {
	path := os.Args[0]
	p, err := filepath.Abs(path)
	if err != nil {
		log.Fatal(err)
	}

	path = filepath.Dir(p)
	ext := filepath.Ext(filepath.Base(p))
	p = strings.TrimSuffix(filepath.Base(p), ext)
	return filepath.Join(path, p)
}

//MakePortsFromSlice {"1-5","9","11:13"} = ["1" "2" "3" "4" "5" "9" "11" "12" "13"]
func MakePortsFromSlice(ps []string) ([]string, error) {
	var res []string
	for _, p := range ps {
		if strings.Contains(p, "-") {
			r, err := makeSlicePort(strings.Split(p, "-"))
			if err != nil {
				return nil, err
			}
			res = append(res, r...)
		}
		if strings.Contains(p, ":") {
			r, err := makeSlicePort(strings.Split(p, ":"))
			if err != nil {
				return nil, err
			}
			res = append(res, r...)
		}
		port, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		res = append(res, strconv.Itoa(port))
	}
	return res, nil
}

func makeSlicePort(s []string) ([]string, error) {
	var res []string
	if len(s) == 2 {
		start, err := strconv.Atoi(s[0])
		if err != nil {
			return nil, err
		}
		end, err := strconv.Atoi(s[1])
		if err != nil {
			return nil, err
		}
		if start > end {
			return nil, errors.New("bad slice " + strings.Join(s, "-"))
		}
		for i := start; i <= end; i++ {
			res = append(res, strconv.Itoa(i))
		}
		return res, nil
	}
	return nil, errors.New("bad slice")
}
