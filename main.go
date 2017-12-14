package main

import (
	"bufio"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/stianeikeland/go-rpio"
)

const camUrl = "/cam.jpg"

var (
	pump = rpio.Pin(21)
)

func waterPumpStatus() string {
	switch pump.Read() {
	case rpio.Low:
		return "Off"
	default:
		return "On"
	}
}

var indexTpl = template.Must(template.New("index.html").Parse(`
<html>
<head>
</head>
<body>
  <div class="control-panel">
	<span>{{.Status}}</span>
  </div>
  <div>
    <img src="{{.ImageUrl}}" />
  </div>
</body>
</html>
`))

func serveCameraStill(w http.ResponseWriter, r *http.Request) {
	tmpImg := filepath.Join(os.TempDir(), camUrl)
	takeImg := exec.Command("/opt/vc/bin/raspistill", "-o", tmpImg)

	// FIXME: Paralellism Bug - Should use local buffer
	takeImg.Stdout = os.Stdout
	takeImg.Stderr = os.Stderr

	// FIXME: Paralellism Bug - Should mutex around execution
	err := takeImg.Run()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	f, err := os.OpenFile(tmpImg, os.O_RDONLY, 0555)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "image/jpeg")

	b := bufio.NewWriter(w)
	_, err = io.Copy(b, f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	b.Flush()
}

type IndexPage struct {
	ImageUrl string
	Status   string
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	b := bufio.NewWriter(w)
	err := indexTpl.Execute(b, IndexPage{camUrl, waterPumpStatus()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	b.Flush()
}

func main() {
	log.Println("waterd starting up")

	log.Println("rpio opening...")
	if err := rpio.Open(); err != nil {
		log.Fatal(err)
	}
	defer rpio.Close()
	log.Println("rpio open success!")

	pump.Output()
	log.Println("rpio: pin 21 set to output mode")

	http.HandleFunc(camUrl, serveCameraStill)
	http.HandleFunc("/", serveIndex)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
