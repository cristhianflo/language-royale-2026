package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/bytedance/sonic"
)

type Event struct {
	TS         *string  `json:"ts"`
	CameraID   *string  `json:"camera_id"`
	Plate      *string  `json:"plate"`
	VIN8       *string  `json:"vin8"`
	Zip        *string  `json:"zip"`
	Confidence *float64 `json:"confidence"`
}

type TopPlate struct {
	Plate    string `json:"plate"`
	HitCount int    `json:"hit_count"`
}

type TopZip struct {
	Zip      string `json:"zip"`
	HitCount int    `json:"hit_count"`
}

type CameraStat struct {
	CameraID          string  `json:"camera_id"`
	TotalHits         int     `json:"total_hits"`
	AverageConfidence float64 `json:"average_confidence"`
}

type BusiestMinute struct {
	Minute   string `json:"minute"`
	HitCount int    `json:"hit_count"`
}

type Output struct {
	TopPlates      []TopPlate      `json:"top_plates"`
	TopZips        []TopZip        `json:"top_zips"`
	CameraStats    []CameraStat    `json:"camera_stats"`
	BusiestMinutes []BusiestMinute `json:"busiest_minutes"`
}

type Report struct {
	StatusCode int     `json:"status_code"`
	Output     *Output `json:"output"`
	Error      string  `json:"error,omitempty"`
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func fastValidateTS(ts string) bool {
	if len(ts) != 20 {
		return false
	}
	if ts[4] != '-' || ts[7] != '-' || ts[10] != 'T' || ts[13] != ':' || ts[16] != ':' || ts[19] != 'Z' {
		return false
	}

	for i := range 20 {
		if i == 4 || i == 7 || i == 10 || i == 13 || i == 16 || i == 19 {
			continue
		}
		if ts[i] < '0' || ts[i] > '9' {
			return false
		}
	}
	return true
}

func validate(e *Event) string {
	if e.TS == nil || !fastValidateTS(*e.TS) {
		return "ts_invalid"
	}

	if e.CameraID == nil || strings.TrimSpace(*e.CameraID) == "" {
		return "camera_id_invalid"
	}

	if e.Plate == nil || strings.TrimSpace(*e.Plate) == "" {
		return "plate_invalid"
	}

	if e.VIN8 == nil || len(strings.TrimSpace(*e.VIN8)) != 8 {
		return "vin8_invalid"
	}

	if e.Zip == nil || len(*e.Zip) != 5 || !isDigits(*e.Zip) {
		return "zip_invalid"
	}

	if e.Confidence == nil {
		return "confidence_type"
	}

	if *e.Confidence < 0.0 || *e.Confidence > 1.0 {
		return "confidence_range"
	}

	return ""
}

func roundToFourDecimals(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func main() {
	inputPath := flag.String("input", "", "input NDJSON file")
	flag.Parse()

	if *inputPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -input <file>\n", os.Args[0])
		os.Exit(1)
	}

	file, err := os.Open(*inputPath)
	if err != nil {
		reportError("file_not_found")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// 1MB buffer for long lines
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	plateCounts := make(map[string]int)
	zipCounts := make(map[string]int)
	minuteCounts := make(map[string]int)

	type camAggr struct {
		hits int
		conf float64
	}
	cameraStatsMap := make(map[string]*camAggr)

	emptyRecordCount := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		if line[0] != '{' {
			reportError("record_not_object")
			return
		}

		var e Event
		if err := sonic.Unmarshal(line, &e); err != nil {
			reportError("json_decode_error")
			return
		}

		emptyRecordCount++

		if reason := validate(&e); reason != "" {
			reportError(reason)
			return
		}

		// Valid event
		plateCounts[*e.Plate]++
		zipCounts[*e.Zip]++

		// "2026-04-03T17:02:11Z" -> "2026-04-03T17:02:00Z"
		// TS is guaranteed valid by validate()
		minKey := (*e.TS)[:16] + ":00Z"
		minuteCounts[minKey]++

		cam := *e.CameraID
		if stat, ok := cameraStatsMap[cam]; ok {
			stat.hits++
			stat.conf += *e.Confidence
		} else {
			cameraStatsMap[cam] = &camAggr{
				hits: 1,
				conf: *e.Confidence,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		reportError("json_decode_error")
		return
	}

	if emptyRecordCount == 0 {
		reportError("empty_input")
		return
	}

	// Aggregation Phase
	out := &Output{
		TopPlates:      make([]TopPlate, 0, len(plateCounts)),
		TopZips:        make([]TopZip, 0, len(zipCounts)),
		CameraStats:    make([]CameraStat, 0, len(cameraStatsMap)),
		BusiestMinutes: make([]BusiestMinute, 0, len(minuteCounts)),
	}

	for p, c := range plateCounts {
		out.TopPlates = append(out.TopPlates, TopPlate{Plate: p, HitCount: c})
	}
	sort.Slice(out.TopPlates, func(i, j int) bool {
		if out.TopPlates[i].HitCount != out.TopPlates[j].HitCount {
			return out.TopPlates[i].HitCount > out.TopPlates[j].HitCount
		}
		return out.TopPlates[i].Plate < out.TopPlates[j].Plate
	})
	if len(out.TopPlates) > 10 {
		out.TopPlates = out.TopPlates[:10]
	}

	for z, c := range zipCounts {
		out.TopZips = append(out.TopZips, TopZip{Zip: z, HitCount: c})
	}
	sort.Slice(out.TopZips, func(i, j int) bool {
		if out.TopZips[i].HitCount != out.TopZips[j].HitCount {
			return out.TopZips[i].HitCount > out.TopZips[j].HitCount
		}
		return out.TopZips[i].Zip < out.TopZips[j].Zip
	})
	if len(out.TopZips) > 10 {
		out.TopZips = out.TopZips[:10]
	}

	for m, c := range minuteCounts {
		out.BusiestMinutes = append(out.BusiestMinutes, BusiestMinute{Minute: m, HitCount: c})
	}
	sort.Slice(out.BusiestMinutes, func(i, j int) bool {
		if out.BusiestMinutes[i].HitCount != out.BusiestMinutes[j].HitCount {
			return out.BusiestMinutes[i].HitCount > out.BusiestMinutes[j].HitCount
		}
		return out.BusiestMinutes[i].Minute < out.BusiestMinutes[j].Minute
	})
	if len(out.BusiestMinutes) > 5 {
		out.BusiestMinutes = out.BusiestMinutes[:5]
	}

	for id, stat := range cameraStatsMap {
		avgConf := roundToFourDecimals(stat.conf / float64(stat.hits))
		out.CameraStats = append(out.CameraStats, CameraStat{
			CameraID:          id,
			TotalHits:         stat.hits,
			AverageConfidence: avgConf,
		})
	}
	sort.Slice(out.CameraStats, func(i, j int) bool {
		return out.CameraStats[i].CameraID < out.CameraStats[j].CameraID
	})

	rep := Report{
		StatusCode: 200,
		Output:     out,
	}

	b, _ := sonic.Marshal(rep)
	fmt.Println(string(b))
}

func reportError(reason string) {
	rep := Report{
		StatusCode: 400,
		Error:      reason,
	}
	b, _ := sonic.Marshal(rep)
	fmt.Println(string(b))
}
