package DataSourceManager

import (
	"encoding/json"
	"log"
	"os"
)

var storagePath = "dataSources.json"

type DataSource struct {
	Name      string
	Idx       uint
	Type      string
	Subsystem string
}

type dsList []DataSource

func (s dsList) Save() error {
	f, err := os.OpenFile(storagePath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(s)
}

var DataSources dsList

var DefaultDataSource = []DataSource{
	DataSource{Name: "ADC1", Idx: 0, Type: "single", Subsystem: "ADC"},
	DataSource{Name: "ADC2", Idx: 1, Type: "single", Subsystem: "ADC"},
	DataSource{Name: "ADC3", Idx: 2, Type: "single", Subsystem: "ADC"},
	DataSource{Name: "ADC4", Idx: 3, Type: "single", Subsystem: "ADC"},
	DataSource{Name: "ADC5", Idx: 4, Type: "single", Subsystem: "ADC"},
	DataSource{Name: "ADC6", Idx: 5, Type: "single", Subsystem: "ADC"},
	DataSource{Name: "ADC7", Idx: 6, Type: "single", Subsystem: "ADC"},
	DataSource{Name: "ADC8", Idx: 7, Type: "single", Subsystem: "ADC"},
}

func init() {
	f, err := os.OpenFile("dataSources.json", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(f)
	err = dec.Decode(DataSources)
	var dsReset = false
	if err != nil {
		dsReset = true
		DataSources = DefaultDataSource
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	if dsReset {
		DataSources.Save()
	}
}

func DataSourceList() ([]DataSource, error) {
	return DataSources, nil
}
