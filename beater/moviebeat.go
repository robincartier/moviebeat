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
	print("\033[H\033[2J")
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

		//List of files to use : https://datasets.imdbws.com/
		//name.basics
		//print(HandleFile("name.basics", bt, b))
		//title.akas
		//print(HandleFile("title.akas", bt, b))
		//title.basics : 
		print(HandleFile("title.basics", bt, b))
		//title.crew
		//print(HandleFile("title.crew", bt, b))
		//title.episode
		//print(HandleFile("title.episode", bt, b))
		//title.principals
		//print(HandleFile("title.principals", bt, b))
		//title.ratings : 
		print(HandleFile("title.ratings", bt, b))
	}
}

// Stop stops moviebeat.
func (bt *Moviebeat) Stop() {
	bt.client.Close()
	close(bt.done)
}

func HandleFile(name string, bt *Moviebeat, b *beat.Beat) (error) {
	var err error

	// Get tsv data from IMDB website
	fileUrl := "https://datasets.imdbws.com/"+name+".tsv.gz"
	logp.Info("Downloading "+fileUrl+" ...")
	err = DownloadFile("/tmp/"+name+".tsv.gz", fileUrl)
	logp.Info(name+".tsv.gz downloaded")

	// Extract file from tsv.gz
	logp.Info("Extracting "+fileUrl+" ...")
	content, err := ExtractGzip("/tmp/"+name+".tsv.gz", "/tmp/"+name+".tsv")
	if err != nil {
		return err
	}
	logp.Info(name+".tsv extracted")


	// Get last read line inside the IMDB file
	// (stored into last_line_imdb.txt)

	// if file does not exit, create it (f is file)
	if _, err := os.Stat("beater/last_line_files/"+name+".txt"); os.IsNotExist(err) {
		f, err := os.Create("beater/last_line_files/"+name+".txt")
		d := []byte("0000000")
		_, err = f.Write(d)
		if err != nil {
			return err
		}
		f.Sync()
		f.Close()
	}
	f, err := os.OpenFile("beater/last_line_files/"+name+".txt", os.O_RDWR, 0666)
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
	logp.Info("Last read line %d for file "+name+ "\n ", last_read_line)

	counter := 0
	for _, line := range strings.Split(content, "\n") {

		// send new data to elastic
		if counter >= last_read_line {
			// One line = One movie/actor
			data_line := strings.Split(string(line), "\t")
			if(len(data_line) >= 3) {
				//Format JSON
				json := createJSON(b.Info.Name, data_line, name)
				event := beat.Event{
					Timestamp: time.Now(),
					Fields: json,
				}
				//Publish to ES
				bt.client.Publish(event)
				logp.Info("Event sent")
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
	// Close file
	f.Sync()
	f.Close()

	logp.Info("Closed "+ name+ " file in /tmp. Write last read line : %s", last_count)

	// remove file
	err = deleteFile("/tmp/"+name+".tsv.gz")
	if err != nil {
		return err
	}
	logp.Info("Removed "+ name+ " file in /tmp")

	return nil
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

func createJSON(beat_info_name string, data_line []string, file_name string) (common.MapStr){
	//Depending of the file, json export is different
	switch file_name {
	case "title.basics":
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
		// Optional fields : (see _meta/fields.yml)
		addIfContent(&json, "title.original", data_line[3])
		addIfContentType(&json, "imdb.genres", data_line[8], "str_array")
		addIfContentType(&json, "duration", data_line[7], "int")
		addIfContentType(&json, "start_year", data_line[5], "int")
		addIfContentType(&json, "end_year", data_line[6], "int")
		return json
	
	case "title.ratings":
		json := common.MapStr{
			"type": beat_info_name,
			"_id": data_line[0],
			"imdb.id": data_line[0],
			"imdb.rating": data_line[1],
			"imdb.numVotes": data_line[2],
		}
		return json
	}

	return nil
}

func addIfContent(m *common.MapStr, key string, value string) {
	if(value != "\\N"){
		(*m)[key] = value
	}
}

func addIfContentType(m *common.MapStr, key string, value string, t string) {
	if(value != "\\N"){
		if(t == "int") {
			v, err := strconv.Atoi(value)
			if err != nil {
	       return
	    }
			(*m)[key] = v
		}
		if(t == "str_array") {
			v := strings.Split(string(value), ",")
			(*m)[key] = v
		}
	}
}
