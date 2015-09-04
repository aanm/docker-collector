package db

import (
	"strings"

	uc "github.com/cilium-team/docker-collector/utils/comm"

	"github.com/cilium-team/docker-collector/Godeps/_workspace/src/github.com/op/go-logging"
)

var log = logging.MustGetLogger("docker-collector")

const DEFAULT_DB = "elasticsearch"

const (
	ContainersTableName        = "containers"
	NetworkInterfacesTableName = "network_interfaces"
	NetworkStatsTableName      = "network_stats"
	NodeTableName              = "node_stats"
	DBDrivers                  = "elasticsearch"
)

func IsValidDBDriver(dbDriver string) bool {
	for _, str := range strings.Split(DBDrivers, "|") {
		if dbDriver == str {
			return true
		}
	}
	return false
}

func InitDb(dbType, indexName, configPath string) error {
	switch dbType {
	case "elasticsearch":
		return InitElasticDb(indexName, configPath)
	default:
		return InitElasticDb(indexName, configPath)
	}
}

func NewConn(indexName, configPath string) (Db, error) {
	return NewConnOf(DEFAULT_DB, indexName, configPath)
}

func NewConnOf(dbType string, indexName string, configPath string) (Db, error) {
	switch dbType {
	case "elasticsearch":
		return NewElasticConn(indexName, configPath)
	default:
		return NewElasticConn(indexName, configPath)
	}
}

type Db interface {
	Close()
	UpdateNode(*uc.Node) error
	CreateNode(*uc.Node) error
	CreateCluster() error
}
