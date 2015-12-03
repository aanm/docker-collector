package db

import (
	"encoding/json"
	l "log"
	"net"
	"os"
	"sync"
	"time"

	uc "github.com/cilium-team/docker-collector/utils/comm"

	"github.com/cilium-team/docker-collector/Godeps/_workspace/src/gopkg.in/olivere/elastic.v3"
)

type logstashConn struct {
	lsConn net.Conn
}

type LogConn struct {
	*elastic.Client
	*logstashConn
	indexName  string
	configPath string
}

type ENode struct {
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type EContainer struct {
	IsActive  bool
	DockerID  string
	Name      string
	NodeName  string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type ENetworkInterface struct {
	Name string
}

type ENetworkStat struct {
	Value                int64
	Name                 string
	ContainerDockerID    string
	ContainerName        string
	NodeName             string
	NetworkInterfaceName string
	UpdatedAt            time.Time
}

const (
	indexFormatString   = `-2006-01-02`
	logNameTimeFormat   = time.RFC3339
	elasticDefaultPort  = "9200"
	elasticDefaultIP    = "127.0.0.1"
	elasticDefaultIndex = "docker-collector"
	logstashDefaultPort = "8080"
	logstashDefaultIP   = "logstash"
)

var (
	ec         LogConn
	clientInit sync.Once
)

func InitElasticDb(indexName, configPath string) error {
	c, err := NewElasticConn(indexName, configPath)
	if err != nil {
		return err
	}
	defer c.Close()

	//	if _, err = c.DeleteIndex(c.indexName + `-*`).Do(); err != nil {
	//		return err
	//	}
	currIndexName := c.indexName + time.Now().Format(indexFormatString)
	iExists, err := c.IndexExists(currIndexName).Do()
	if err != nil {
		return err
	}
	if !iExists {
		if _, err = c.CreateIndex(currIndexName).Do(); err != nil {
			return err
		}
	}
	if err = setMappings(c); err != nil {
		return err
	}
	return nil
}

func setMappings(c LogConn) error {
	mappingPro := map[string]interface{}{
		NodeTableName: map[string]interface{}{
			"properties": map[string]interface{}{
				"ContainerName": map[string]string{
					"type":  "string",
					"index": "not_analyzed",
				},
				"NodeName": map[string]string{
					"type":  "string",
					"index": "not_analyzed",
				},
				"NetworkInterfaceName": map[string]string{
					"type":  "string",
					"index": "not_analyzed",
				},
				"Name": map[string]string{
					"type":  "string",
					"index": "not_analyzed",
				},
			},
		},
	}
	_, err := c.PutMapping().IgnoreConflicts(true).IgnoreUnavailable(true).Index(c.indexName + `*`).Type(NodeTableName).BodyJson(mappingPro).Do()
	return err
}

func NewElasticConn(indexName string, configPath string) (LogConn, error) {
	log.Debug("")
	elasticPort := os.Getenv("ELASTIC_PORT")
	if elasticPort == "" {
		elasticPort = elasticDefaultPort
	}
	elasticIP := os.Getenv("ELASTIC_IP")
	if elasticIP == "" {
		elasticIP = elasticDefaultIP
	}
	logstashPort := os.Getenv("LOGSTASH_PORT")
	if logstashPort == "" {
		logstashPort = logstashDefaultPort
	}
	logstashIP := os.Getenv("LOGSTASH_IP")
	if logstashIP == "" {
		logstashIP = logstashDefaultIP
	}
	if indexName == "" {
		indexName = elasticDefaultIndex
	}
	return NewConnTo(elasticIP, elasticPort, logstashIP, logstashPort, indexName, configPath)
}

func NewConnTo(elasticIP, elasticPort, logstashIP, logstashPort, indexName, configPath string) (LogConn, error) {
	log.Debug("")
	var outerr error
	clientInit.Do(func() {
		logTimename := time.Now().Format(logNameTimeFormat)
		fo, err := os.Create(os.TempDir() + "/docker-collector-elastic-out-" + logTimename + ".log")
		if err != nil {
			l.Fatalf("Error while creating a log file: %s", err)
		}
		fe, err := os.Create(os.TempDir() + "/docker-collector-elastic-error-" + logTimename + ".log")
		if err != nil {
			l.Fatalf("Error while creating a log file: %s", err)
		}
		//		ft, err := os.Create(os.TempDir() + "/docker-collector-elastic-trace-" + logTimename + ".log")
		//		if err != nil {
		//			l.Fatalf("Error while creating a log file: %s", err)
		//		}
		l.Printf("Trying to connect to ElasticSearch to '%s':'%s'\n", elasticIP, elasticPort)

		ec.Client, outerr = elastic.NewClient(
			elastic.SetURL("http://"+elasticIP+":"+elasticPort),
			elastic.SetMaxRetries(10),
			elastic.SetHealthcheckTimeoutStartup(30*time.Second),
			elastic.SetSniff(false),
			elastic.SetErrorLog(l.New(fe, "", l.LstdFlags)),
			elastic.SetInfoLog(l.New(fo, "", l.LstdFlags)),
			//elastic.SetTraceLog(l.New(ft, "", l.LstdFlags)),
		)
		if outerr == nil {
			l.Printf("Success!\n")
		} else {
			l.Printf("Error %+v\n", outerr)
		}
		ec.indexName = indexName
		ec.configPath = configPath
		lc := logstashConn{}
		ec.logstashConn = &lc
		if outerr == nil {
			outerr = ec.connectToLogstash(logstashIP + ":" + logstashPort)
		}

	})
	return ec, outerr
}

func (c LogConn) connectToLogstash(logstashAddr string) error {
	var outerr error
	retries := 0
	// Give it one minute
	for retries < 12 {
		l.Printf("Trying to connect to Logstash to '%s'\n", logstashAddr)
		c.lsConn, outerr = net.Dial("tcp", logstashAddr)
		if outerr == nil {
			l.Printf("Success!\n")
			break
		} else {
			retries++
			time.Sleep(5 * time.Second)
			l.Printf("Error %+v\n", outerr)
		}
	}
	return outerr
}

func (c LogConn) reconnectToLogstash() error {
	rAddr := c.lsConn.RemoteAddr()
	return c.connectToLogstash(rAddr.String())
}

func (c LogConn) Close() {
}

func convertToElasticNetStat(cont uc.Container, netInt uc.NetworkInterface, stat uc.NetworkStat) ENetworkStat {
	return ENetworkStat{
		Value:                stat.CurrentValue,
		Name:                 stat.Name,
		ContainerDockerID:    cont.DockerID,
		ContainerName:        cont.NodeName + cont.Name,
		NodeName:             cont.NodeName,
		NetworkInterfaceName: netInt.Name,
	}
}

func (c LogConn) UpdateNode(node *uc.Node) error {
	now := time.Now()
	node.UpdatedAt = now
	for _, cont := range node.Containers {
		cont.UpdateLastValue()
		for _, inter := range cont.NetworkInterfaces {
			for _, stat := range inter.NetworkStats {
				enetstat := convertToElasticNetStat(cont, inter, stat)
				enetstat.UpdatedAt = now
				enetstatBytes, err := json.Marshal(enetstat)
				if err != nil {
					log.Error("error while marshalling '%+v': \"%v\"", enetstat, err)
				}
				enetstatBytes = append(enetstatBytes, '\n')
				_, err = c.lsConn.Write(enetstatBytes)
				if err != nil {
					log.Error("error while sending bytes to logstash '%+v': \"%v\"", enetstat, err)
					if err := c.reconnectToLogstash(); err != nil {
						log.Error("Fail to reconnect: %#v", err)
					}
				}
			}
		}
	}
	return nil
}
