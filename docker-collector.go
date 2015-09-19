package main

import (
	"flag"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	u "github.com/cilium-team/docker-collector/utils"
	uc "github.com/cilium-team/docker-collector/utils/comm"
	ucdb "github.com/cilium-team/docker-collector/utils/comm/db"

	"github.com/cilium-team/docker-collector/Godeps/_workspace/src/github.com/op/go-logging"
	d "github.com/cilium-team/docker-collector/Godeps/_workspace/src/github.com/samalba/dockerclient"
)

var (
	logLevel      string
	refreshTime   uint64
	dbDriver      string
	skipRegFilter string
	indexName     string
	configPath    string
	log           = logging.MustGetLogger("docker-collector")
)

func init() {
	flag.StringVar(&skipRegFilter, "f", "", "Regex option to prevent docker-collector from reading on those containers that are matched by the given regex. Example: docker-collector -f docker-*")
	flag.StringVar(&logLevel, "l", "info", "Set log level, valid options are (debug|info|warning|error|fatal|panic)")
	flag.Uint64Var(&refreshTime, "t", 60, "Set refresh time (in seconds) to retrieve statistics from containers")
	flag.StringVar(&dbDriver, "d", "elasticsearch", "Set database driver to store statistics, valid options are ("+ucdb.DBDrivers+")")
	flag.StringVar(&indexName, "i", "docker-collector", "Use a specific the prefix of the index name for elasticsearch. Suffix is -YYYY-MM-DD")
	flag.StringVar(&configPath, "c", "/docker-collector/configs", "Directory path for kibana configuration and or templates. Configuration filename: 'configs.json', template filename: 'templates.json'")
	flag.Parse()
	setupLOG()

	if refreshTime == 0 {
		log.Fatal("Refresh time must be a number greater than 0 (zero)")
		return
	}
	if !ucdb.IsValidDBDriver(dbDriver) {
		log.Fatalf("Invalid database driver. Valid options are: \"%s\"", ucdb.DBDrivers)
		return
	}
}

func setupLOG() {
	var (
		stdout = logging.MustStringFormatter(
			"%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}",
		)
		fileFormat = logging.MustStringFormatter(
			"%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x} %{message}",
		)
	)
	level, err := logging.LogLevel(logLevel)
	if err != nil {
		log.Fatal(err)
	}
	fo, err := os.Create(os.TempDir() + "/docker-collector-" + strconv.FormatInt(time.Now().Unix(), 10) + ".log")
	fileBackend := logging.NewLogBackend(fo, "", 0)
	stdoutBackend := logging.NewLogBackend(os.Stdout, "", 0)

	fBF := logging.NewBackendFormatter(fileBackend, fileFormat)
	sBF := logging.NewBackendFormatter(stdoutBackend, stdout)

	backendLeveled := logging.SetBackend(fBF, sBF)
	backendLeveled.SetLevel(level, "")
	logging.SetBackend(backendLeveled)
}

var containersMutex = &sync.Mutex{}

func main() {
	db, err := ucdb.NewConnOf(dbDriver, indexName, configPath)
	if err != nil {
		log.Error("Error: %s", err)
		return
	}
	if err = ucdb.InitDb(dbDriver, indexName, configPath); err != nil {
		log.Error("Error: %s", err)
		return
	}
	defer db.Close()
	dockerS, err := uc.NewDockerClientSamalba()
	if err != nil {
		log.Error("Error: %s", err)
		return
	}

	log.Info("Trying to get docker client info")
	wait := 1 * time.Second
	retries := 10
	for {
		log.Info("Attempt %d...", 11-retries)
		if _, err := dockerS.Info(); err == nil {
			break
		}
		if retries < 0 {
			log.Error("Unable to monitor for events on the given docker client")
			return
		}
		time.Sleep(wait)
		wait += wait
		retries--
	}
	log.Info("Connection successful")

	docker, err := uc.NewDockerClient()
	if err != nil {
		log.Error("Error: %s", err)
		return
	}

	containers := NewContainersRegistry(docker, db)

	dockerAPIContainers, err := dockerS.ListContainers(true, false, "")
	if err != nil {
		log.Error("Error: %s", err)
		return
	}

	dockerS.StartMonitorEvents(listenForEvents, nil, docker, containers)

	for _, dockerContainer := range dockerAPIContainers {
		matches := false
		if skipRegFilter != "" {
			for _, cName := range dockerContainer.Names {
				if match, _ := regexp.MatchString(skipRegFilter, cName); match {
					matches = true
					break
				}
			}
		}
		if !matches {
			containers.Create(dockerContainer.Id)
		}
	}

	log.Info("docker-collector has started")

	//Discard first reading
	containersMutex.Lock()
	readContainers(docker, containers)
	for _, cont := range containers.Node.Containers {
		cont.UpdateLastValue()
	}
	if err := db.CreateCluster(); err != nil {
		log.Error("error while creating cluster for kibana: %+v", err)
	}
	if err := db.CreateNode(&containers.Node); err != nil {
		log.Error("error while creating a node for kibana: %+v", err)
	}
	containersMutex.Unlock()

	for {
		timeToProcess1 := time.Now()
		containersMutex.Lock()
		if containers.ActiveContainers() != 0 {
			readContainers(docker, containers)
			if err := containers.UpdateDBNode(); err != nil {
				log.Error("Error while updating node: %v", err)
			}
		}
		containersMutex.Unlock()
		timeToProcess2 := time.Now()
		time.Sleep(time.Second*time.Duration(refreshTime) - timeToProcess2.Sub(timeToProcess1))
	}
}

func readContainers(docker uc.Docker, containers *ContainersRegistry) {
	for i, container := range containers.Node.Containers {
		if container.IsActive {
			containers.Node.Containers[i].UpdateNetInterfaces()
			for _, netInterface := range container.NetworkInterfaces {
				if netInterface.IsActive {
					for j, networkStat := range netInterface.NetworkStats {
						intpath := uc.BuildNetworkIntPath(netInterface.Name, networkStat.Name)
						value, err := u.ReadContainerFromProc(container.PID, intpath)
						if err != nil {
							value = "0"
						}
						intVal, err := strconv.ParseInt(value, 10, 64)
						if err != nil {
							log.Error("Error while formating value '%s' to int", value)
						}
						netInterface.NetworkStats[j].ValueRead = intVal
						log.Debug("Container: %s %s/%s: %s", container.DockerID, netInterface.Name, networkStat.Name, value)
					}
				}
			}
		}
	}
}

func listenForEvents(event *d.Event, ec chan error, args ...interface{}) {
	if event != nil {
		docker := args[0].(uc.Docker)
		containers := args[1].(*ContainersRegistry)
		go func(event d.Event) {
			log.Debug("Msg received %s", event)
			containersMutex.Lock()
			switch event.Status {
			case "create":
				break
			case "start":
				if dic, err := docker.InspectContainer(event.Id); err == nil {
					matches := false
					if skipRegFilter != "" {
						if match, _ := regexp.MatchString(skipRegFilter, dic.Name); match {
							matches = true
						}
					}
					if !matches {
						log.Info("Container '%s' added to audit", dic.Name)
						strpid := strconv.Itoa(dic.State.Pid)
						containers.Activate(event.Id, strpid)
						if err := containers.UpdateDBNode(); err != nil {
							log.Error("Error while updating node: %v", err)
						}
					}
				}
			case "stop":
				if containers.Deactivate(event.Id) {
					log.Info("Container '%s' paused from audit", event.Id)
					if err := containers.UpdateDBNode(); err != nil {
						log.Error("Error while updating node: %v", err)
					}
				}
			case "destroy":
				fallthrough
			case "die":
				if i := containers.GetSliceIndex(event.Id); i != -1 {
					log.Info("Container '%s' removed from audit", containers.Node.Containers[i].Name)
					containers.DeleteByIndex(i)
					if err := containers.UpdateDBNode(); err != nil {
						log.Error("Error while updating node: %v", err)
					}
				}
			}
			containersMutex.Unlock()
		}(*event)
	}
}
