package SessionManager

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/wdreeveii/DataCollector/DataSourceManager"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	//"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"time"
)

type ClientSub struct {
	Out      chan struct{ X, Y float64 }
	stopChan chan chan bool
}

func (cs *ClientSub) DumpToFile() {
	log.Println("Entering DumpToFile")

	for {
		select {
		case msg := <-cs.Out:
			log.Println("DumpToFile Recv", msg)

		case doneChan := <-cs.stopChan:
			doneChan <- true
			log.Println("Exiting DumpToFile")
			return
		}
	}
}

type DSSub struct {
	Name       DataSourceManager.DSName
	SourceSub  *DataSourceManager.DataSourceSubscription
	ClientSubs []ClientSub
	stopChan   chan chan bool
}

func (dss *DSSub) Stop() {
	log.Println("Trying Stop()")
	var s = make(chan bool)
	dss.stopChan <- s
	<-s
}

func (dss *DSSub) FanOut(s *Session) {
	log.Println("Entering FanOut")
	for {
		select {
		case msg := <-dss.SourceSub.Out:
			//log.Println("FanOut SourceSub", msg)
			s.data[dss.Name] = append(s.data[dss.Name], msg)
			for _, v := range dss.ClientSubs {
				select {
				case v.Out <- msg:
				default:
				}
			}
		case doneChan := <-dss.stopChan:
			dss.SourceSub.Unsubscribe()

			for _, v := range dss.ClientSubs {
				var dc = make(chan bool)
				v.stopChan <- dc
				<-dc
			}
			doneChan <- true
			log.Println("Exiting FanOut")
			return
		}
	}
}

type Session struct {
	Name                string
	DT                  string
	CapturedDataSources []DataSourceManager.DSName
	data                map[DataSourceManager.DSName][]struct{ X, Y float64 }
	Recording           bool
	sourceSubs          map[DataSourceManager.DSName]*DSSub
}

func (s Session) Save() error {
	session_path := path.Join(datapath, s.DT+"."+s.Name)
	err := os.MkdirAll(session_path, os.ModeDir|os.ModePerm)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path.Join(session_path, "manifest.json"), os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(s)
}

func (s *Session) Record(en bool) error {
	if s.sourceSubs == nil {
		s.sourceSubs = make(map[DataSourceManager.DSName]*DSSub)
	}
	s.Recording = en
	if en {
		for _, v := range s.CapturedDataSources {
			var dss DSSub
			var err error
			dss.Name = v
			dss.SourceSub, err = DataSourceManager.SubscribeFeed(v)
			if err != nil {
				return err
			}

			dss.stopChan = make(chan chan bool)

			go dss.FanOut(s)

			var cs ClientSub
			cs.Out = make(chan struct{ X, Y float64 })
			cs.stopChan = make(chan chan bool)
			dss.ClientSubs = []ClientSub{cs}

			go cs.DumpToFile()

			s.sourceSubs[v] = &dss
		}
	} else {
		log.Println("Cleaning up all sources")
		for _, v := range s.sourceSubs {
			v.Stop()
		}
		//s.sourceSubs = nil
	}
	return nil
}

type ByDT []Session

func (a ByDT) Len() int           { return len(a) }
func (a ByDT) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByDT) Less(i, j int) bool { return a[j].DT < a[i].DT }

var Sessions []Session
var datapath = "data/"
var nameregex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var fileregex = regexp.MustCompile(`^(20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}[+-]{1}[0-9]{2}:[0-9]{2}).([a-zA-Z0-9_-]+)$`)

func init() {
	rand.Seed(int64(0))
	Sessions = make([]Session, 0)
	err := os.MkdirAll(datapath, os.ModeDir|os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	files, err := ioutil.ReadDir(datapath)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() {
			matches := fileregex.FindStringSubmatch(file.Name())
			if len(matches) > 0 {
				f, err := os.OpenFile(path.Join(datapath, file.Name(), "manifest.json"), os.O_RDWR, 0755)
				if err != nil {
					log.Println(err)
					continue
				}
				dec := json.NewDecoder(f)
				var t Session
				err = dec.Decode(&t)
				if err != nil {
					log.Println(err)
					f.Close()
					continue
				}
				t.data = make(map[DataSourceManager.DSName][]struct{ X, Y float64 })
				Sessions = append(Sessions, t)
			}
		}
	}
}

func SessionList(start, end uint) ([]Session, error) {
	lend := len(Sessions) - int(start)
	lstart := lend - int(end)

	if lstart < 0 {
		lstart = 0
	}
	if lend < 0 {
		lend = 0
	}
	sess := Sessions[lstart:lend]
	sort.Sort(ByDT(sess))
	return sess, nil
}

func RecentSessions(limit uint) ([]Session, error) {
	start := len(Sessions) - 10
	if start < 0 {
		start = 0
	}
	end := len(Sessions)

	sess := Sessions[start:end]
	sort.Sort(ByDT(sess))

	return sess, nil
}

func NewSession(t *Session) error {
	if !nameregex.MatchString(t.Name) {
		return fmt.Errorf("Name paramater contains invalid characters")
	}
	t.DT = time.Now().Format(time.RFC3339)
	err := t.Save()
	if err != nil {
		return err
	}
	t.data = make(map[DataSourceManager.DSName][]struct{ X, Y float64 })
	Sessions = append(Sessions, *t)
	return nil
}

func RemoveSession(t *Session) error {
	for k, s := range Sessions {
		if t.Name == s.Name && t.DT == s.DT {
			Sessions = append(Sessions[:k], Sessions[k+1:]...)
			err := os.RemoveAll(path.Join(datapath, t.DT+"."+t.Name))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type SessionDetail struct {
	CapturedDataSources []DataSourceManager.DSName
	Recording           bool
}

func SessionDetails(t *Session) (*SessionDetail, error) {
	for _, s := range Sessions {
		if t.Name == s.Name && t.DT == s.DT {
			var d = SessionDetail{CapturedDataSources: s.CapturedDataSources, Recording: s.Recording}
			return &d, nil
		}
	}
	return nil, fmt.Errorf("Session not found")
}

func SessionControl(t *Session, captureEnabled bool) error {
	for k, s := range Sessions {
		if t.Name == s.Name && t.DT == s.DT {
			if s.Recording != captureEnabled {
				return Sessions[k].Record(captureEnabled)
			}
			return nil
		}
	}
	return fmt.Errorf("Session not found")
}

func PlotStream(w io.Writer, notify <-chan bool, t *Session) error {
	// Make sure that the writer supports flushing.
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("Streaming unsupported on this client")
	}

	var cs ClientSub
	cs.Out = make(chan struct{ X, Y float64 })
	cs.stopChan = make(chan chan bool)
	for _, s := range Sessions {
		if t.Name == s.Name && t.DT == s.DT {
			s.sourceSubs[t.CapturedDataSources[0]].ClientSubs = append(s.sourceSubs[t.CapturedDataSources[0]].ClientSubs, cs)
		}
	}

	for {
		select {
		case <-cs.Out:
			log.Println("Plotting...")
			var b bytes.Buffer
			Plot(bufio.NewWriter(&b), t)
			w.Write([]byte("event: message\n"))
			for {
				line, err := b.ReadBytes('\n')
				if len(line) > 0 {
					w.Write([]byte("data:"))
					w.Write(line)
				}
				if err != nil {
					if err != io.EOF {
						log.Println("Error transmitting plot", err)
					}
					break
				}
			}
			w.Write([]byte("\n\n"))
			flusher.Flush()
		case doneChan := <-cs.stopChan:
			doneChan <- true
			log.Println("Closing (stopChan) PlotStream")
			return nil
		case <-notify:
			log.Println("Closing (notify) PlotStream")
			return nil
		}
	}

	return nil
}

// xticks defines how we convert and display time.Time values.
var xticks = plot.TimeTicks{Format: "2006-01-02\n15:04"}

func Plot(w io.Writer, t *Session) error {
	var data plotter.XYs
	var ok bool
	for _, s := range Sessions {
		if t.Name == s.Name && t.DT == s.DT {
			data, ok = s.data[t.CapturedDataSources[0]]
			if !ok {
				// try loading from file
				data = plotter.XYs{}
			}
		}
	}

	if data == nil {
		return fmt.Errorf("Session not found")
	}

	p, err := plot.New()
	if err != nil {
		return err
	}

	p.Title.Text = string(t.CapturedDataSources[0])
	p.X.Tick.Marker = xticks
	p.X.Label.Text = "DT"
	p.Y.Label.Text = "Y"
	p.Add(plotter.NewGrid())

	l, err := plotter.NewLine(data)
	if err != nil {
		return err
	}

	l.LineStyle.Width = vg.Points(1)
	p.Add(l)
	p.Legend.Add(string(t.CapturedDataSources[0]), l)

	/*err = plotutil.AddLinePoints(p, string(t.CapturedDataSources[0]), data)
	if err != nil {
		return err
	}*/

	// Save the plot to a PNG file.
	writer, err := p.WriterTo(10*vg.Inch, 5*vg.Inch, "svg")
	if err != nil {
		return err
	}

	_, err = writer.WriteTo(w)
	return err
}
