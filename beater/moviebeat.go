package beater

import (
	"time"
  "os"
  "io"
  "fmt"
  "strconv"
  "strings"
	"net/http"
	"compress/gzip"
	"io/ioutil"

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

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(bt.config.Period)
	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
		}

		logp.Info("Starting new beat loop")

		// Get tsv data from IMDB website
		fileUrl := "https://datasets.imdbws.com/title.basics.tsv.gz"
    err = DownloadFile("/tmp/imdb.tsv.gz", fileUrl)

		logp.Info("IMDB file downloaded")

		content, err := ExtractGzip("/tmp/imdb.tsv.gz", "/tmp/imdb.tsv")
		if err != nil {
        return err
    }

		// Get last read line inside the IMDB file
		// (stored into last_line_imdb.txt)

		// if file does not exit, create it
		if _, err := os.Stat("beater/last_line_imdb.txt"); os.IsNotExist(err) {
			f, err := os.Create("beater/last_line_imdb.txt")
			d := []byte("0000000")
			_, err = f.Write(d)
			if err != nil {
			  return err
			}
			f.Sync()
			f.Close()
		}
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

		logp.Info("Last read line %d\n", last_read_line)

		counter := 0
		for _, line := range strings.Split(content, "\n") {

			// send new data to elastic
			if counter >= last_read_line {
				data_line := strings.Split(string(line), "\t")
				if(len(data_line) >= 9) {
					json := createJSON(b.Info.Name, data_line)
					event := beat.Event{
						Timestamp: time.Now(),
						Fields: json,
					}
					bt.client.Publish(event)
					// logp.Info("Event sent")
				}
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

		logp.Info("Write last read line : %s", last_count)

		// remove IMDB file
		err = deleteFile("/tmp/imdb.tsv.gz")
		if err != nil {
	    return err
		}

		logp.Info("Remove imdb file in /tmp")
	}
}

// Stop stops moviebeat.
func (bt *Moviebeat) Stop() {
	bt.client.Close()
	close(bt.done)
}


func DownloadFile(filepath string, url string) error {

    // Create the file
    out, err := os.Create(filepath)
    if err != nil {
        return err
    }
    defer out.Close()

    // Get the data
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // Write the body to file
    _, err = io.Copy(out, resp.Body)
    if err != nil {
        return err
    }

    return nil
}

func ExtractGzip(archive string, output string) (string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return "",err
	}

	reader, err := gzip.NewReader(f)
	if err != nil {
		return "",err
	}

	result, err := ioutil.ReadAll(reader)
	if err != nil {
		return "",err
	}
	// io.Copy(output, reader)
	// reader.Close()

	return string(result), nil
}

func deleteFile(path string) error {
	return os.Remove(path)
}

func createJSON(beat_info_name string, data_line []string) (common.MapStr){
	json := common.MapStr{
		"type": beat_info_name,
		"imdb.id": data_line[0],
		"imdb.type": data_line[1],
		"title.primary": data_line[2],
	}

	if(data_line[4] == "1"){
		json["is_adult"] = true
	} else {
		json["is_adult"] = false
	}
	// Optional fields : start_year, end_year
	// (see _meta/fields.yml)
	addIfContent(&json, "title.original", data_line[3])
	addIfContent(&json, "duration", data_line[7])
	addIfContent(&json, "start_year", data_line[5])
	addIfContent(&json, "end_year", data_line[6])
	addIfContent(&json, "imdb.genres", data_line[8])

	return json
}

func addIfContent(m *common.MapStr, key string, value string) {
	if(value != "\\N"){
		(*m)[key] = value
	}
}
