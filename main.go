package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	log.Printf("%v\n", os.Args)
	http.HandleFunc("/marco", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Header().Set("Content-type", "text/plain")
		w.Header().Set("X-Marco-Version", "0.0.3")
		fmt.Fprintf(w, "POLO!\n")
	})

	http.Handle("/", newGopherTilesHandler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "text/plain")
		fmt.Fprintf(w, "yes good\n")
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}

func newGopherTilesHandler() http.Handler {
	const gopherURL = "https://blog.golang.org/go-programming-language-turns-two_gophers.jpg"
	res, err := http.Get(gopherURL)
	if err != nil {
		log.Fatal(err)
	}
	if res.StatusCode != 200 {
		log.Fatalf("Error fetching %s: %v", gopherURL, res.Status)
	}
	slurp, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	im, err := jpeg.Decode(bytes.NewReader(slurp))
	if err != nil {
		if len(slurp) > 1024 {
			slurp = slurp[:1024]
		}
		log.Fatalf("Failed to decode gopher image: %v (got %q)", err, slurp)
	}

	type subImager interface {
		SubImage(image.Rectangle) image.Image
	}
	const tileSize = 32
	xt := im.Bounds().Max.X / tileSize
	yt := im.Bounds().Max.Y / tileSize
	var tile [][][]byte // y -> x -> jpeg bytes
	for yi := 0; yi < yt; yi++ {
		var row [][]byte
		for xi := 0; xi < xt; xi++ {
			si := im.(subImager).SubImage(image.Rectangle{
				Min: image.Point{xi * tileSize, yi * tileSize},
				Max: image.Point{(xi + 1) * tileSize, (yi + 1) * tileSize},
			})
			buf := new(bytes.Buffer)
			if err := jpeg.Encode(buf, si, &jpeg.Options{Quality: 90}); err != nil {
				log.Fatal(err)
			}
			row = append(row, buf.Bytes())
		}
		tile = append(tile, row)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms, _ := strconv.Atoi(r.FormValue("latency"))
		const nanosPerMilli = 1e6
		if r.FormValue("x") != "" {
			x, _ := strconv.Atoi(r.FormValue("x"))
			y, _ := strconv.Atoi(r.FormValue("y"))
			if ms <= 1000 {
				time.Sleep(time.Duration(ms) * nanosPerMilli)
			}
			if x >= 0 && x < xt && y >= 0 && y < yt {
				http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(tile[y][x]))
				return
			}
		}
		io.WriteString(w, "<html><body>")
		fmt.Fprintf(w, "A grid of %d tiled images is below. Compare:<p>", xt*yt)
		/*for _, ms := range []int{0, 30, 200, 1000} {
			d := time.Duration(ms) * nanosPerMilli
			fmt.Fprintf(w, "[<a href='https://%s/gophertiles?latency=%d'>HTTP/2, %v latency</a>] [<a href='http://%s/gophertiles?latency=%d'>HTTP/1, %v latency</a>]<br>\n",
				httpsHost(), ms, d,
				httpHost(), ms, d,
			)
		} */
		io.WriteString(w, "<p>\n")
		cacheBust := time.Now().UnixNano()
		for y := 0; y < yt; y++ {
			for x := 0; x < xt; x++ {
				fmt.Fprintf(w, "<img width=%d height=%d src='/gophertiles?x=%d&y=%d&cachebust=%d&latency=%d'>",
					tileSize, tileSize, x, y, cacheBust, ms)
			}
			io.WriteString(w, "<br/>\n")
		}
		io.WriteString(w, "<hr><a href='/'>&lt;&lt Back to Go HTTP/2 demo server</a></body></html>")
	})
}
