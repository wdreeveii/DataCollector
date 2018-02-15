package DataSourceManager

import (
	"encoding/json"
	"fmt"
	"github.com/wdreeveii/goADS1256"
	"log"
	"os"
	"time"
)

var storagePath = "dataSources.json"

type Driver struct {
	Enable   bool
	Sources  map[DSName]*DataSource
	stopChan chan chan bool
}

func (d *Driver) Start() error {
	d.stopChan = make(chan chan bool)

	err := goADS1256.Open()
	if err != nil {
		return err
	}

	go d.collect()

	d.Enable = true
	return nil
}

func (d *Driver) Stop() {
	var doneChan = make(chan bool)
	d.stopChan <- doneChan
	<-doneChan
}

func (d *Driver) collect() {
	defer goADS1256.Close()

	log.Println("Entering Driver")
	for {
		select {
		case <-time.After(time.Second):
			var value struct{ X, Y float64 }
			value.X = float64(time.Now().Unix())
			var haveSubscribers bool
			for _, v := range d.Sources {
				if v.out != nil {
					haveSubscribers = true
					value.Y = float64(goADS1256.Sample(0, uint8(v.Idx)))
					log.Println("Sending Data to DS", v)
					select {
					case v.out <- value:
					default:
					}
				}
			}
			if !haveSubscribers {
				d.stopChan = nil
				d.Enable = false
				log.Println("Exiting Driver")
				return
			}
		case doneChan := <-d.stopChan:
			doneChan <- true
			log.Println("Exiting Driver")
			return
		}
	}
}

type DSName string

type DataSource struct {
	Name       DSName
	Idx        uint
	Type       string
	Subsystem  string
	out        chan struct{ X, Y float64 }
	clientSubs []*DataSourceSubscription
	unsub      chan *DataSourceSubscription
}

type DataSourceSubscription struct {
	Out   chan struct{ X, Y float64 }
	unsub chan *DataSourceSubscription
}

func (dss *DataSourceSubscription) Unsubscribe() {
	dss.unsub <- dss
}

func (ds *DataSource) Subscribe() (*DataSourceSubscription, error) {
	if ds.unsub == nil {
		ds.unsub = make(chan *DataSourceSubscription)
		ds.out = make(chan struct{ X, Y float64 })
		go ds.FanOut()
	}
	var dss DataSourceSubscription
	dss.Out = make(chan struct{ X, Y float64 })
	dss.unsub = ds.unsub
	ds.clientSubs = append(ds.clientSubs, &dss)
	return &dss, nil
}

func (ds *DataSource) FanOut() {
	log.Println("Entering FanOut", ds.Name)
	for {
		select {
		case msg := <-ds.out:
			//log.Println("FanOut", ds.Name, "recv", msg, ds.clientSubs)
			for _, v := range ds.clientSubs {
				select {
				case v.Out <- msg:
				default:
				}
			}
		case dss := <-ds.unsub:
			for k, v := range ds.clientSubs {
				if v == dss {
					ds.clientSubs = append(ds.clientSubs[:k], ds.clientSubs[k+1:]...)
				}
			}

			if len(ds.clientSubs) == 0 {
				ds.out = nil
				ds.unsub = nil
				log.Println("Exiting FanOut", ds.Name)
				return
			}

		}
	}
}

type dsInfo struct {
	Drivers map[string]*Driver
}

func (s dsInfo) Save() error {
	f, err := os.OpenFile(storagePath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(s)
}

var DataSources dsInfo

var DefaultDataSource = dsInfo{Drivers: map[string]*Driver{"ADC": &Driver{
	Sources: map[DSName]*DataSource{
		"ADC1": &DataSource{Name: "ADC1", Idx: 0, Type: "single", Subsystem: "ADC"},
		"ADC2": &DataSource{Name: "ADC2", Idx: 1, Type: "single", Subsystem: "ADC"},
		"ADC3": &DataSource{Name: "ADC3", Idx: 2, Type: "single", Subsystem: "ADC"},
		"ADC4": &DataSource{Name: "ADC4", Idx: 3, Type: "single", Subsystem: "ADC"},
		"ADC5": &DataSource{Name: "ADC5", Idx: 4, Type: "single", Subsystem: "ADC"},
		"ADC6": &DataSource{Name: "ADC6", Idx: 5, Type: "single", Subsystem: "ADC"},
		"ADC7": &DataSource{Name: "ADC7", Idx: 6, Type: "single", Subsystem: "ADC"},
		"ADC8": &DataSource{Name: "ADC8", Idx: 7, Type: "single", Subsystem: "ADC"},
	}}}}

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
	var dsl []DataSource
	for _, v := range DataSources.Drivers {
		for _, vs := range v.Sources {
			dsl = append(dsl, *vs)
		}
	}
	return dsl, nil
}

func getSubsystem(dsName DSName) (string, error) {
	for k, v := range DataSources.Drivers {
		for _, vs := range v.Sources {
			if dsName == vs.Name {
				return k, nil
			}
		}
	}
	return "", fmt.Errorf("Datasource %s not found", dsName)
}

func SubscribeFeed(dsName DSName) (*DataSourceSubscription, error) {
	subsystem, err := getSubsystem(dsName)
	if err != nil {
		return nil, err
	}

	if !DataSources.Drivers[subsystem].Enable {
		err = DataSources.Drivers[subsystem].Start()
		if err != nil {
			return nil, err
		}
	}

	return DataSources.Drivers[subsystem].Sources[dsName].Subscribe()

}
