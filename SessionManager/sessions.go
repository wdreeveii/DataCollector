package SessionManager

import (
	"encoding/json"
	"fmt"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path"
	"regexp"
	"sort"
	"time"
)

type Session struct {
	Name                string
	DT                  string
	CapturedDataSources []string
	Recording           bool
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

func SessionMetrics(t *Session) ([]string, error) {
	for _, s := range Sessions {
		if t.Name == s.Name && t.DT == s.DT {
			return s.CapturedDataSources, nil
		}
	}
	return nil, fmt.Errorf("Session not found")
}

func Plot(w io.Writer, t *Session) error {

	p, err := plot.New()
	if err != nil {
		return err
	}

	p.Title.Text = t.CapturedDataSources[0]
	p.X.Label.Text = "X"
	p.Y.Label.Text = "Y"

	err = plotutil.AddLinePoints(p, t.CapturedDataSources[0], randomPoints(1500))
	if err != nil {
		return err
	}

	// Save the plot to a PNG file.
	writer, err := p.WriterTo(4*vg.Inch, 4*vg.Inch, "svg")
	if err != nil {
		return err
	}

	_, err = writer.WriteTo(w)
	return err
}

// randomPoints returns some random x, y points.
func randomPoints(n int) plotter.XYs {
	pts := make(plotter.XYs, n)
	for i := range pts {
		if i == 0 {
			pts[i].X = rand.Float64()
		} else {
			pts[i].X = pts[i-1].X + rand.Float64()
		}
		pts[i].Y = pts[i].X + 10*rand.Float64()
	}
	return pts
}
