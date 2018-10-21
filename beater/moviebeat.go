package beater

import (
	"time"
  "os"
	"os/exec"
  "io"
  "fmt"
  "strconv"
  "bufio"
  "strings"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/robincartier/moviebeat/config"
)

// Moviebeat configuration.
type Moviebeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
}

// New creates an instance of moviebeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &Moviebeat{
		done:   make(chan struct{}),
		config: c,
	}
	return bt, nil
}

// Run starts moviebeat.
func (bt *Moviebeat) Run(b *beat.Beat) error {
	logp.Info("moviebeat is running! Hit CTRL-C to stop it.")

	// Get tsv data from IMDB website
	cmd := exec.Command("wget", "https://datasets.imdbws.com/title.basics.tsv.gz")
	if err := cmd.Run(); err != nil {
		return err
	}
	logp.Info("IMDB file downloaded")
  cmd = exec.Command("gunzip", "title.basics.tsv.gz")
  if err := cmd.Run(); err != nil {
    return err
  }
	logp.Info("IMDB file extracted")

	// Get last read line inside the IMDB file
	// (stored into last_line_imdb.txt)
	f, err := os.OpenFile("beater/last_line_imdb.txt", os.O_RDWR, 0666)
	if err != nil {
    return err
	}

	b1 := make([]byte, 7)
	_, err = f.Read(b1)
	if err != nil {
    return err
	}

	last_read_line, err := strconv.Atoi(string(b1))
	if err != nil {
    return err
	}

	fmt.Printf("Last read line %d\n", last_read_line)

	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}


	// open and read IMDB file
	file, err := os.Open("title.basics.tsv")
	if err != nil {
    return err
	}

	defer file.Close()

	reader := bufio.NewReader(file)

	counter := 0
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}

		// send new data to elastic
		if counter >= last_read_line {
			data_line := strings.SplitAfter(string(line), "\t")

			event := beat.Event{
				Timestamp: time.Now(),
				Fields: common.MapStr{
					"type":    b.Info.Name,
					"tconst":	data_line[0],
					"titleType":	data_line[1],
					"primaryTitle":	data_line[2],
					"originalTitle":	data_line[3],
					"isAdult":	data_line[4],
					"startYear":	data_line[5],
					"endYear":	data_line[6],
					"runtimeMinutes":	data_line[7],
					"genres":	data_line[8],
				},
			}
			bt.client.Publish(event)
			logp.Info("Event sent")
		}
		counter++
	}
	// Write last read line into last_line_imdb.txt
  last_count := fmt.Sprintf("%07d\n", counter)
  _, err = f.Seek(0, 0)
   if err != nil {
    return err
	}
  d := []byte(last_count)
  _, err = f.Write(d)
  if err != nil {
    return err
	}
  f.Sync()
	f.Close()

	// remove IMDB file
	cmd = exec.Command("rm", "title.basics.tsv.gz")
  if err := cmd.Run(); err != nil {
    return err
  }
	return nil
}

// Stop stops moviebeat.
func (bt *Moviebeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
