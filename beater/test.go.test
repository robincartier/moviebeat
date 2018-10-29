package main

import (
	"log"
  "os"
	"os/exec"
  "io"
  "fmt"
  "strconv"
  "bufio"
  "strings"
)



func check(e error) {
  if e != nil {
      log.Fatal(e)
  }
}


func main() {
	cmd := exec.Command("wget", "https://datasets.imdbws.com/title.basics.tsv.gz")
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
  cmd = exec.Command("gunzip", "title.basics.tsv.gz")
  if err := cmd.Run(); err != nil {
    log.Fatal(err)
  }

  f, err := os.OpenFile("last_line_imdb.txt", os.O_RDWR, 0666)

  check(err)

  defer f.Close()

  b := make([]byte, 7)
  n, err := f.Read(b)
  check(err)

  last_read_line, err := strconv.Atoi(string(b))
  check(err)
  fmt.Printf("%d bytes read, last read line %d\n", n, last_read_line)



  file, err := os.Open("title.basics.tsv")
  check(err)

  defer file.Close()

  reader := bufio.NewReader(file)

  counter := 0
  for {
    line, _, err := reader.ReadLine()

    if err == io.EOF {
      break
    }
    if counter >= last_read_line {
      result := strings.SplitAfter(string(line), "\t")

      fmt.Println(result[0])
    }
    counter++
  }
  fmt.Printf("last counter %d\n", counter)
  last_count := fmt.Sprintf("%07d\n", counter)
  _, err = f.Seek(0, 0)
   check(err)
  d := []byte(last_count)
  _, err = f.Write(d)
  check(err)

  f.Sync()
  f.Close()

  cmd = exec.Command("rm", "title.basics.tsv.gz")
  if err := cmd.Run(); err != nil {
    log.Fatal(err)
  }
}
