package main

import (
	"os"
	"time"

	uc "github.com/cilium-team/docker-collector/utils/comm"
	ucdb "github.com/cilium-team/docker-collector/utils/comm/db"
)

type ContainersRegistry struct {
	DB   ucdb.Db
	Node uc.Node
}

func NewContainersRegistry(dClient uc.Docker, db ucdb.Db) *ContainersRegistry {
	hn, err := os.Hostname()
	if err != nil {
		log.Debug("Error while getting the hostname: %v", err)
	}
	return &ContainersRegistry{
		Node: uc.Node{
			DockerClient: dClient,
			CreatedAt:    time.Now(),
			Name:         hn,
		},
		DB: db,
	}
}

func (c *ContainersRegistry) UpdateDBNode() error {
	return c.DB.UpdateNode(&c.Node)
}

func (c *ContainersRegistry) GetSliceIndex(dockerID string) int {
	return c.Node.GetSliceIndex(dockerID)
}

func (c *ContainersRegistry) Create(dockerID string) error {
	if err := c.Node.Create(dockerID); err != nil {
		return err
	}
	return c.DB.UpdateNode(&c.Node)
}

func (c *ContainersRegistry) DeleteByIndex(i int) {
	c.Node.Containers = append(c.Node.Containers[:i], c.Node.Containers[i+1:]...)
	c.DB.UpdateNode(&c.Node)
}

func (c *ContainersRegistry) DeleteByDockerId(dockerID string) {
	if i := c.GetSliceIndex(dockerID); i != -1 {
		c.DeleteByIndex(i)
	}
}

func (c *ContainersRegistry) Activate(dockerID, dockerPID string) {
	c.Node.Activate(dockerID, dockerPID)
}

func (c *ContainersRegistry) Deactivate(dockerID string) bool {
	return c.Node.Deactivate(dockerID)
}

func (c *ContainersRegistry) ActiveContainers() int {
	return c.Node.ActiveContainers()
}
