package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gps_clients/server_gps_service/config"
	"gps_clients/server_gps_service/utils"

	"golang.org/x/sys/windows/svc"
)

func usage(errmsg string) {
	fmt.Fprintf(os.Stderr,
		"%s\n\n"+
			"usage: %s <command>\n"+
			"       where <command> is one of\n"+
			"       install, remove, debug, start, stop, pause or resume.\n",
		errmsg, os.Args[0])
	os.Exit(2)
}

func main() {
	if err := config.ReadConfig(utils.GetProgramPath() + ".json"); err != nil {
		log.Fatal(err)
	}

	svcName := config.Config.ServiceName
	if svcName == "" {
		svcName = "tlka_gps_service"
	}

	nameInstallService := strings.ReplaceAll(svcName, "_", " ")
	descInstallService := config.Config.DescService

	inService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("failed to determine if we are running in service: %v", err)
	}

	if inService {
		runService(svcName, false)
		return
	}

	if len(os.Args) < 2 {
		usage("no command specified")
	}

	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "debug":
		runService(svcName, true)
		return
	case "install":
		err = installService(svcName, nameInstallService, descInstallService)
	case "remove":
		err = removeService(svcName)
	case "start":
		err = startService(svcName)
	case "stop":
		err = controlService(svcName, svc.Stop, svc.Stopped)
	case "pause":
		err = controlService(svcName, svc.Pause, svc.Paused)
	case "resume":
		err = controlService(svcName, svc.Continue, svc.Running)
	default:
		usage(fmt.Sprintf("invalid command %s", cmd))
	}
	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, svcName, err)
	}
}
