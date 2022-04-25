package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

var elog debug.Log

type myservice struct{}

func (m *myservice) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	//startExecute()
loop:
	for {
		//select {
		/*case*/
		c := <-r //:
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
			time.Sleep(100 * time.Millisecond)
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			testOutput := strings.Join(args, "-")
			testOutput += fmt.Sprintf("-%d", c.Context)
			elog.Info(1, testOutput)
			//AddToLog(GetProgramPath()+"-test.txt", testOutput)
			stopServers()
			break loop
		case svc.Pause:
			//stopExecute()
			changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
			//ioutil.WriteFile("D:/test.txt", []byte("pause"), 0777)
			//AddToLog(GetProgramPath()+"-test.txt", "start pause service")
			stopServers()
			//AddToLog(GetProgramPath()+"-test.txt", "service paused")
		case svc.Continue:
			changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			//startExecute()
			//ioutil.WriteFile("D:/test.txt", []byte("continue"), 0777)
			//AddToLog(GetProgramPath()+"-test.txt", "start continue service")
			startServers()
			//AddToLog(GetProgramPath()+"-test.txt", "service continued")
		default:
			elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			//AddToLog(GetProgramPath()+"-test.txt", fmt.Sprintf("unexpected control request #%d", c))
		}
		//}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	initServer()
	elog.Info(1, fmt.Sprintf("starting %s service", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &myservice{})
	if err != nil {
		elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}

	elog.Info(1, fmt.Sprintf("%s service stopped", name))
}
