package comm

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	ue "github.com/cilium-team/docker-collector/utils/executable"
)

// Statistics based on: https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-class-net-statistics
const (
	netStatsBasePath = "/sys/class/net/"
	stats            = "/statistics/"
)

var NetStatsNames = []string{
	"collisions",
	"multicast",
	"rx_bytes",
	"rx_compressed",
	"rx_crc_errors",
	"rx_dropped",
	"rx_fifo_errors",
	"rx_frame_errors",
	"rx_length_errors",
	"rx_missed_errors",
	"rx_over_errors",
	"rx_packets",
	"tx_aborted_errors",
	"tx_bytes",
	"tx_carrier_errors",
	"tx_compressed",
	"tx_dropped",
	"tx_errors",
	"tx_fifo_errors",
	"tx_heartbeat_errors",
	"tx_packets",
	"tx_window_errors",
}

type Node struct {
	//	NumberOfActCont int
	Name         string
	DockerClient Docker `json:"-",sql:"-"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
	Containers   []Container
}

type Container struct {
	ID                int `json:"-"`
	PID               int `json:"-",sql:"-"`
	DockerID          string
	Name              string
	NodeName          string
	NetworkInterfaces []NetworkInterface
	IsActive          bool `sql:"-"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

type NetworkInterface struct {
	ID           int `json:"-"`
	ContainerID  int `json:"-",sql:"index"`
	Name         string
	IsActive     bool `json:"-"`
	NetworkStats []NetworkStat
}

type NetworkStat struct {
	ID                 int `json:"-"`
	NetworkInterfaceID int `json:"-",sql:"index"`
	Name               string
	ExecID             string `json:"-",sql:"-"`
	ValueRead          int64  `json:"-",sql:"-"`
	LastValueRead      int64  `json:"-",sql:"-"`
	CurrentValue       int64
}

func (n *Node) GetSliceIndex(dockerID string) int {
	log.Debug("")
	for i, container := range n.Containers {
		if container.DockerID == dockerID {
			return i
		}
	}
	return -1
}

func (n *Node) Activate(dockerID, dockerPID string) {
	log.Debug("")
	if i := n.GetSliceIndex(dockerID); i != -1 {
		if netInter, err := createNetworkInterfaces(dockerPID); err == nil {
			n.Containers[i].IsActive = true
			n.Containers[i].AddNewInterfaces(netInter)
			if num, err := strconv.Atoi(dockerPID); err != nil {
				n.Containers[i].PID = num
			} else {
				n.Containers[i].PID = 0
			}
		}
	} else {
		n.Create(dockerID)
	}
}

func (n *Node) Deactivate(dockerID string) bool {
	log.Debug("")
	if i := n.GetSliceIndex(dockerID); i != -1 {
		n.Containers[i].IsActive = false
		return true
	} else {
		return false
	}
}

func (n *Node) ActiveContainers() int {
	log.Debug("")
	var count int
	for _, container := range n.Containers {
		if container.IsActive {
			count++
		}
	}
	return count
}

func createNetworkInterfaces(dockerPID string) ([]NetworkInterface, error) {
	log.Debug("")
	netInterNames, err := listLocalNetInt(dockerPID)
	if err != nil {
		return nil, err
	}
	var networkInterfaces []NetworkInterface
	for _, netInterName := range netInterNames {
		netInt := NetworkInterface{
			Name: netInterName,
		}
		for _, netStatName := range NetStatsNames {
			networkStat := NetworkStat{
				Name: netStatName,
			}
			netInt.NetworkStats = append(netInt.NetworkStats, networkStat)
		}
		networkInterfaces = append(networkInterfaces, netInt)
	}
	return networkInterfaces, nil
}

func listLocalNetInt(pid string) ([]string, error) {
	log.Debug("")
	stdout, err := ue.ExecShCommand(`ls /proc/` + pid + `/root` + netStatsBasePath)
	if err != nil {
		return nil, err
	}
	stdoutstr := string(stdout)
	var interfaces []string
	if len(stdoutstr) != 0 {
		if strings.Contains(stdoutstr, "executable file not found") {
			log.Debug("stdout %s", stdout)
			return nil, errors.New("Doesn't support network statistics")
		}
		for _, inter := range strings.Split(stdoutstr, "\n") {
			if len(inter) != 0 {
				interfaces = append(interfaces, inter)
			}
		}
	}
	return interfaces, nil
}

func (n *Node) Create(dockerID string) error {
	log.Debug("")
	inspectCont, err := n.DockerClient.InspectContainer(dockerID)
	if err != nil {
		return err
	}
	networkInterfaces, err := createNetworkInterfaces(strconv.Itoa(inspectCont.State.Pid))
	if err != nil {
		return err
	}
	hn, err := os.Hostname()
	if err != nil {
		log.Error("Error while getting the host name: %v", err)
	}
	container := Container{
		NetworkInterfaces: networkInterfaces,
		IsActive:          true,
		DockerID:          dockerID,
		Name:              inspectCont.Name,
		NodeName:          hn,
		PID:               inspectCont.State.Pid,
	}
	n.Containers = append(n.Containers, container)
	return nil
}

func (cont *Container) UpdateLastValue() {
	log.Debug("")
	for _, netInter := range cont.NetworkInterfaces {
		for j, _ := range netInter.NetworkStats {
			netInter.NetworkStats[j].CurrentValue,
				netInter.NetworkStats[j].LastValueRead =
				netInter.NetworkStats[j].ValueRead-netInter.NetworkStats[j].LastValueRead,
				netInter.NetworkStats[j].ValueRead
		}

	}
}

func (cont *Container) AddNewInterfaces(newNetInterfaces []NetworkInterface) {
	log.Debug("")
	for i := range cont.NetworkInterfaces {
		cont.NetworkInterfaces[i].IsActive = false
	}
	for _, newNetInter := range newNetInterfaces {
		gotActive := false
		for i := range cont.NetworkInterfaces {
			if newNetInter.Name == cont.NetworkInterfaces[i].Name {
				log.Debug("Activating %s", newNetInter.Name)
				cont.NetworkInterfaces[i].IsActive = true
				gotActive = true
				break
			}
		}
		if !gotActive {
			log.Info("Adding interface '%s' to '%s'", newNetInter.Name, cont.Name)
			newNetInter.IsActive = true
			cont.NetworkInterfaces = append(cont.NetworkInterfaces, newNetInter)
		}
	}
}

func (cont *Container) UpdateNetInterfaces() {
	log.Debug("")
	if inter, err := createNetworkInterfaces(strconv.Itoa(cont.PID)); err == nil {
		cont.AddNewInterfaces(inter)
	}
}

func BuildNetworkIntPath(netIntName, statName string) string {
	log.Debug("netIntName: %v", netIntName)
	log.Debug("statName: %v", statName)
	return netStatsBasePath + netIntName + stats + statName
}
