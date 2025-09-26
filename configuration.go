package main

import (
	"log"
	"time"

	"github.com/encratite/commons"
	"gopkg.in/yaml.v3"
)

const (
	configurationPath = "configuration/configuration.yaml"
	timeLayout = "2006-01-02 15:04"
)

type Configuration struct {
	Source string `yaml:"source"`
	Races []RaceConfiguration `yaml:"races"`
}

type RaceConfiguration struct {
	Path string `yaml:"path"`
	Practice *SerializableTime `yaml:"practice"`
	Qualifying *SerializableTime `yaml:"qualifying"`
	Race *SerializableTime `yaml:"race"`
}

type SerializableTime struct {
	time.Time
}

var configuration *Configuration

func loadConfiguration() {
	if configuration != nil {
		panic("Configuration had already been loaded")
	}
	yamlData := commons.ReadFile(configurationPath)
	configuration = new(Configuration)
	err := yaml.Unmarshal(yamlData, configuration)
	if err != nil {
		log.Fatal("Failed to unmarshal YAML:", err)
	}
	configuration.validate()
}

func (c *Configuration) validate() {
	if c.Source == "" {
		log.Fatalf("Source missing from configuration file")
	}
	for _, race := range c.Races {
		race.validate()
	}
}

func (r *RaceConfiguration) validate() {
	if r.Path == "" {
		log.Fatalf("Path missing from race configuration")
	}
	times := []*SerializableTime{
		r.Practice,
		r.Qualifying,
		r.Race,
	}
	for _, t := range times {
		if t == nil {
			log.Fatalf("Missing timestamp in race configuration: %s", r.Path)
		}
	}
	if !r.Practice.Before(r.Qualifying.Time) || !r.Qualifying.Before(r.Race.Time) {
		log.Fatalf("Invalid times in race configuration: %s", r.Path)
	}
}

func (d *SerializableTime) UnmarshalYAML(value *yaml.Node) error {
	timestamp, err := time.Parse(timeLayout, value.Value)
	if err != nil {
		log.Fatalf("Failed to parse timestamp: %s", value.Value)
	}
	d.Time = timestamp
	return nil
}