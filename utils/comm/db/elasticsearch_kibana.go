package db

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	uc "github.com/cilium-team/docker-collector/utils/comm"

	"github.com/cilium-team/docker-collector/Godeps/_workspace/src/gopkg.in/olivere/elastic.v3"
)

const (
	maxSizeXKibanaDashboard = 12 + 1
	maxSizeYKibanaDashboard = 1023 + 1
	configsFilename         = `configs.json`
	templateFilename        = `templates.json`
	kibanaIndex             = `.kibana`
	defaultDashboardName    = `docker-collector`
)

type elasticBody struct {
	Index  string                 `json:"_index"`
	Type   string                 `json:"_type"`
	ID     string                 `json:"_id"`
	Source map[string]interface{} `json:"_source"`
}

type panel struct {
	Id      string   `json:"id"`
	Type    string   `json:"type"`
	Col     int      `json:"col"`
	Row     int      `json:"row"`
	Size_x  int      `json:"size_x"`
	Size_y  int      `json:"size_y"`
	Columns []string `json:"columns,omitempty"`
	Sort    []string `json:"sort,omitempty"`
}

// intersects checks if one of the panels is intersecting the other panel.
func (p panel) intersects(other panel) bool {
	return !((other.Col+other.Size_x <= p.Col) ||
		(other.Row+other.Size_y <= p.Row) ||
		(other.Col >= p.Col+p.Size_x) ||
		(other.Row >= p.Row+p.Size_y))
}

type kibanaSavedObjectMeta struct {
	SearchSourceJSON string `json:"searchSourceJSON"`
}

type kibanaStruct struct {
	Title                 string                `json:"title"`
	Hits                  int64                 `json:"hits,omitempty"`
	Description           string                `json:"description"`
	Version               int64                 `json:"version"`
	KibanaSavedObjectMeta kibanaSavedObjectMeta `json:"kibanaSavedObjectMeta"`
}

type kibanaDashboard struct {
	PanelsJSON string  `json:"panelsJSON"`
	Panels     []panel `json:"-"`
	kibanaStruct
}

func (c LogConn) CreateCluster() error {
	configs, err := readConfigFile(c.configPath + string(filepath.Separator) + configsFilename)
	if err != nil {
		return err
	}

	cKCfg, err := c.createSingleIS("config", configs)
	if err != nil {
		return err
	}
	if res, err := cKCfg.Do(); err != nil {
		return err
	} else {
		// If version != 1 means that the cluster was already created
		if res.Version != 1 {
			log.Info("Cluster already created, moving on...")
			return nil
		}
	}

	cIP, err := c.createBulkIS("index-pattern", configs)
	if err != nil {
		return err
	}
	if _, err := cIP.Do(); err != nil {
		return err
	}

	cS, err := c.createBulkIS("search", configs)
	if err != nil {
		return err
	}
	if _, err := cS.Do(); err != nil {
		return err
	}

	cV, err := c.createBulkIS("visualization", configs)
	if err != nil {
		return err
	}
	if _, err := cV.Do(); err != nil {
		return err
	}

	cDCfg, err := c.createSingleIS("dashboard", configs)
	if err != nil {
		return err
	}
	if _, err := cDCfg.Do(); err != nil {
		return err
	}

	return nil
}

func (c LogConn) put(dashBoardName string, p panel) (*elastic.IndexResponse, error) {
	getResult, err := c.Get().Index(kibanaIndex).Type("dashboard").Id(dashBoardName).Do()
	if err != nil {
		return nil, err
	}
	var (
		kdb kibanaDashboard
	)
	if getResult.Found {
		if err := json.Unmarshal(*getResult.Source, &kdb); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(kdb.PanelsJSON), &kdb.Panels); err != nil {
			return nil, err
		}
	} else {
		kdb.Title = dashBoardName
		kdb.Panels = []panel{}
		kdb.Version = 1
		kdb.KibanaSavedObjectMeta =
			kibanaSavedObjectMeta{
				SearchSourceJSON: `{"filter":[{"query":{"query_string":{"analyze_wildcard":true,"query":"*"}}}]}`,
			}
	}
	pos_x, pos_y := fittablePos(kdb.Panels, p)
	if pos_x == -1 && pos_y == -1 {
		return nil, nil
	}
	p.Col = pos_x
	p.Row = pos_y
	kdb.Panels = append(kdb.Panels, p)
	b, err := json.Marshal(&kdb.Panels)
	if err != nil {
		return nil, err
	}
	kdb.PanelsJSON = string(b)
	return c.Index().Index(kibanaIndex).Type("dashboard").Refresh(true).Id(dashBoardName).BodyJson(kdb).Do()
}

func fittablePos(panels []panel, tempPanel panel) (int, int) {
	if len(panels) == 0 {
		return 1, 1
	}
	for _, p := range panels {
		if p.Id == tempPanel.Id {
			return -1, -1
		}
	}
	for tempPanel.Row = 1; tempPanel.Row < maxSizeYKibanaDashboard; tempPanel.Row++ {
		for tempPanel.Col = 1; tempPanel.Col < maxSizeXKibanaDashboard; tempPanel.Col++ {
			overlaps := false
			for _, p := range panels {
				if p.intersects(tempPanel) {
					overlaps = true
					tempPanel.Col += p.Size_x - 1
					break
				}
			}
			if !overlaps &&
				tempPanel.Col+tempPanel.Size_x <= maxSizeXKibanaDashboard &&
				tempPanel.Row+tempPanel.Size_y <= maxSizeYKibanaDashboard {
				return tempPanel.Col, tempPanel.Row
			}
		}
	}
	return 1, tempPanel.Row
}

func replaceNodeNameOnSearch(e *elasticBody, name string) {
	e.ID = strings.Replace(e.ID, `$NodeName$`, name, -1)
	if v, ok := e.Source["title"]; ok {
		e.Source["title"] = strings.Replace(v.(string), `$NodeName$`, name, -1)
	}
	if v, ok := e.Source["kibanaSavedObjectMeta"]; ok {
		if v, ok := v.(map[string]interface{})["searchSourceJSON"]; ok {
			e.Source["kibanaSavedObjectMeta"].(map[string]interface{})["searchSourceJSON"] =
				strings.Replace(v.(string), `$NodeName$`, name, -1)
		}
	}
}

func replaceNodeNameOnVisualization(e *elasticBody, name string) {
	e.ID = strings.Replace(e.ID, `$NodeName$`, name, -1)
	if v, ok := e.Source["title"]; ok {
		e.Source["title"] = strings.Replace(v.(string), `$NodeName$`, name, -1)
	}
	if v, ok := e.Source["savedSearchId"]; ok {
		e.Source["savedSearchId"] = strings.Replace(v.(string), `$NodeName$`, name, -1)
	}
}

func (c LogConn) CreateNode(node *uc.Node) error {
	templates, err := readConfigFile(c.configPath + string(filepath.Separator) + templateFilename)
	if err != nil {
		return err
	}

	cS := filterByType("search", templates)

	for i := range cS {
		replaceNodeNameOnSearch(&cS[i], node.Name)
	}

	cBS, err := c.createBulkIS("search", cS)
	if err != nil {
		return err
	}
	if _, err := cBS.Do(); err != nil {
		return err
	}

	cVs := filterByType("visualization", templates)

	for i := range cVs {
		replaceNodeNameOnVisualization(&cVs[i], node.Name)
	}

	cBV, err := c.createBulkIS("visualization", cVs)
	if err != nil {
		return err
	}
	if _, err := cBV.Do(); err != nil {
		return err
	}

	dashBoardName := c.getDashBoardName()

	for _, cV := range cVs {
		p := panel{
			Id:     cV.ID,
			Type:   "visualization",
			Col:    1,
			Row:    1,
			Size_x: 6,
			Size_y: 4,
		}
		if _, err := c.put(dashBoardName, p); err != nil {
			log.Warning("Failed to insert panel %s into dashboard. You can add it manually in kibana.Error: %v", p.Id, err)
			break
		}
	}

	return nil
}

func (c LogConn) createSingleIS(typeStr string, configs []elasticBody) (*elastic.IndexService, error) {
	cfgType := filterByType(typeStr, configs)
	if len(cfgType) == 0 {
		return nil, fmt.Errorf("type '%s' not found in configuration files", typeStr)
	}
	return c.Index().Index(cfgType[0].Index).Type(cfgType[0].Type).Refresh(true).Id(cfgType[0].ID).BodyJson(cfgType[0].Source), nil
}

func (c LogConn) createBulkIS(typeStr string, configs []elasticBody) (*elastic.BulkService, error) {
	cfgType := filterByType(typeStr, configs)

	if len(cfgType) == 0 {
		return nil, fmt.Errorf("type '%s' not found in configuration files", typeStr)
	}
	bulkReq := c.Bulk().Index(cfgType[0].Index).Refresh(true)
	for _, cfg := range cfgType {
		bulkReq.Add(elastic.NewBulkIndexRequest().Index(cfg.Index).
			Type(cfg.Type).Id(cfg.ID).Doc(cfg.Source))
	}
	return bulkReq, nil
}

func readConfigFile(filepath string) ([]elasticBody, error) {
	var eBS []elasticBody
	if b, err := ioutil.ReadFile(filepath); err != nil {
		return nil, err
	} else {
		if err := json.Unmarshal(b, &eBS); err != nil {
			return nil, fmt.Errorf("file '%s' is a malformed configuration file: %s", filepath, err)
		} else {
			log.Info("Found configuration file '%s'", filepath)
		}
	}
	return eBS, nil
}

func filterByType(typeStr string, eBS []elasticBody) []elasticBody {
	var filteredEBS []elasticBody
	for _, eB := range eBS {
		if eB.Type == typeStr {
			filteredEBS = append(filteredEBS, eB)
		}
	}
	return filteredEBS
}

func (c LogConn) getDashBoardName() string {
	dashboardName := defaultDashboardName
	configs, err := readConfigFile(c.configPath + string(filepath.Separator) + configsFilename)
	if err != nil {
		return dashboardName
	}
	cfgs := filterByType("dashboard", configs)
	if len(cfgs) != 0 {
		dashboardName = cfgs[0].ID
	}
	return dashboardName
}
