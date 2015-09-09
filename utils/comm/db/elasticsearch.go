package db

import (
	l "log"
	"os"
	"sync"
	"time"

	uc "github.com/cilium-team/docker-collector/utils/comm"

	"github.com/cilium-team/docker-collector/Godeps/_workspace/src/gopkg.in/olivere/elastic.v2"
)

type EConn struct {
	*elastic.Client
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
)

var (
	ec         EConn
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

func setMappings(c EConn) error {
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

func NewElasticConn(indexName string, configPath string) (EConn, error) {
	log.Debug("")
	port := os.Getenv("ELASTIC_PORT")
	if port == "" {
		port = elasticDefaultPort
	}
	ip := os.Getenv("ELASTIC_IP")
	if ip == "" {
		ip = elasticDefaultIP
	}
	if indexName == "" {
		indexName = elasticDefaultIndex
	}
	return NewElasticConnTo(ip, port, indexName, configPath)
}

func NewElasticConnTo(ip, port, indexName, configPath string) (EConn, error) {
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
		l.Printf("Trying to connect to ElasticSearch to '%s':'%s'\n", ip, port)

		ec.Client, err = elastic.NewClient(
			elastic.SetURL("http://"+ip+":"+port),
			elastic.SetMaxRetries(10),
			elastic.SetHealthcheckTimeoutStartup(30*time.Second),
			elastic.SetSniff(false),
			elastic.SetErrorLog(l.New(fe, "", l.LstdFlags)),
			elastic.SetInfoLog(l.New(fo, "", l.LstdFlags)),
			//elastic.SetTraceLog(l.New(ft, "", l.LstdFlags)),
		)
		if err == nil {
			l.Printf("Success!\n")
		} else {
			l.Printf("Error %+v\n", err)
		}
		ec.indexName = indexName
		ec.configPath = configPath
		outerr = err
	})
	return ec, outerr
}

func (c EConn) Close() {
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

func (c EConn) UpdateNode(node *uc.Node) error {
	now := time.Now()
	node.UpdatedAt = now
	currIndexName := c.indexName + now.Format(indexFormatString)
	bulkReq := c.Bulk().Index(currIndexName).Refresh(true)
	for _, cont := range node.Containers {
		cont.UpdateLastValue()
		for _, inter := range cont.NetworkInterfaces {
			for _, stat := range inter.NetworkStats {
				enetstat := convertToElasticNetStat(cont, inter, stat)
				enetstat.UpdatedAt = now
				bulkReq.Add(elastic.NewBulkIndexRequest().Index(currIndexName).
					Type(NodeTableName).Timestamp(enetstat.UpdatedAt.Format(time.RFC3339Nano)).Doc(enetstat))
			}
		}
	}
	_, err := bulkReq.Do()
	if err != nil {
		return err
	}
	return nil
}
