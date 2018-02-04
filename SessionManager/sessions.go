package SessionManager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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
				s := Session{Name: matches[2], DT: matches[1]}
				Sessions = append(Sessions, s)
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
