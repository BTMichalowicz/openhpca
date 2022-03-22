//
// Copyright (c) 2020-2021, NVIDIA CORPORATION. All rights reserved.
//
// See LICENSE.txt for license information
//

package result

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gvallee/validation_tool/pkg/label"
	"github.com/openucx/openhpca/tools/internal/pkg/overlap"
	"github.com/openucx/openhpca/tools/internal/pkg/score"
	"github.com/openucx/openhpca/tools/internal/pkg/util"
)

const (
	FilePermission = 0666
	FileName       = "openhpca_results.txt"
)

type Data struct {
	Text []string
	Unit string
}

type Results struct {
	resultsDir         string
	osuResultFiles     map[string]string
	smbResultFiles     map[string]string
	overlapResultFiles map[string]string
	OsuFilesMap        map[string]string
	OsuData            map[string]*Data
	overlapFilesMap    map[string]string
	overlapData        map[string]*Data
}

func (r *Results) GetOverlapData() map[string][]string {
	res := make(map[string][]string)
	for benchName, f := range r.overlapResultFiles {
		content, err := ioutil.ReadFile(filepath.Join(r.resultsDir, f))
		if err != nil {
			return nil
		}
		lines := strings.Split(string(content), "\n")
		if len(lines) < 2 {
			log.Printf("%s has an invalid content", f)
			return nil
		}
		res[benchName] = lines

		// We also store the results in the result object
		if r.overlapData == nil {
			r.overlapData = make(map[string]*Data)
		}
		d := new(Data)
		d.Text = lines
		r.overlapData[benchName] = d
		if r.overlapFilesMap == nil {
			r.overlapFilesMap = make(map[string]string)
		}
		r.overlapFilesMap[benchName] = f
	}
	return res
}

// LoadResultsWithPrefix enables loading OSU-type results from a file.
// This is meant to be used to load data from results files of different variants/versions of the OSU benchmark
func (r *Results) LoadResultsWithPrefix(testPrefix string) map[string]*Data {
	res := make(map[string]*Data)
	for benchName, f := range r.osuResultFiles {
		if strings.HasPrefix(benchName, testPrefix) {
			if testPrefix == "osu" && strings.HasPrefix(benchName, "osu_noncontig_mem") {
				continue
			}

			subBenchmark := strings.TrimPrefix(benchName, testPrefix+"_")
			f = filepath.Join(r.resultsDir, f)
			content, err := ioutil.ReadFile(f)
			if err != nil {
				return nil
			}
			text := string(content)
			lines := strings.Split(text, "\n")
			d := new(Data)
			d.Text = lines
			benchKey := testPrefix + "_" + subBenchmark
			res[benchKey] = d

			// We also store the results in the result object
			if r.OsuData == nil {
				r.OsuData = make(map[string]*Data)
			}
			if r.OsuData[benchKey] == nil {
				d := new(Data)
				d.Text = lines
				if subBenchmark == "latency" {
					for _, line := range lines {
						if strings.HasPrefix(line, "# Size Latency (") {
							unit := strings.TrimPrefix(line, "# Size Latency (")
							unit = strings.TrimLeft(unit, "\n")
							unit = strings.TrimSuffix(unit, ")")
							d.Unit = unit
						}
					}
				}
				if subBenchmark == "bw" {
					for _, line := range lines {
						if strings.HasPrefix(line, "# Size Bandwidth (") {
							unit := strings.TrimPrefix(line, "# Size Bandwidth (")
							unit = strings.TrimLeft(unit, "\n")
							unit = strings.TrimSuffix(unit, ")")
							d.Unit = unit
						}
					}
				}
				r.OsuData[benchKey] = d
			}
			if r.OsuFilesMap == nil {
				r.OsuFilesMap = make(map[string]string)
			}

			r.OsuFilesMap[benchKey] = f
		}
	}

	return res
}

func (r *Results) GetLatency() (float32, string, error) {
	unit := "Unknown"
	size := -1.0
	lat := -1.0
	f := r.osuResultFiles["osu_latency"]
	if f == "" {
		return 0, "", fmt.Errorf("unable to get output file for OSU latency")
	}

	f = filepath.Join(r.resultsDir, f)
	content, err := ioutil.ReadFile(f)
	if err != nil {
		return 0, unit, err
	}
	lines := strings.Split(string(content), "\n")
	for idx, line := range lines {
		if idx == 0 {
			// Skip the first line
			continue
		}

		if line == "" || strings.HasPrefix(line, "#") {
			if strings.Contains(line, "Latency (") {
				tokens := strings.Split(line, "Latency (")
				idx := strings.Index(tokens[1], ")")
				unit = tokens[1][:idx]
			}
			continue
		}

		// Parse a real data line
		words := strings.Split(line, " ")
		for _, w := range words {
			if w == "" || w == " " {
				continue
			}
			if size == -1.0 {
				size, err = strconv.ParseFloat(w, 32)
				if err != nil {
					return 0, unit, fmt.Errorf("unable to get actual latency data from %s: %w", w, err)
				}
			} else {
				lat, err = strconv.ParseFloat(w, 32)
				if err != nil {
					return 0, unit, fmt.Errorf("unable to get actual latency data from %s: %w", w, err)
				}
				return float32(lat), unit, nil
			}
		}
	}

	return 0, unit, fmt.Errorf("unable to find result file for latency")
}

func (r *Results) GetSMBOverlap() (float32, error) {
	f := r.smbResultFiles["smb_mpi_overhead"]
	if f == "" {
		return 0, fmt.Errorf("unable to get output file for SMB MPI overhead")
	}

	f = filepath.Join(r.resultsDir, f)
	content, err := ioutil.ReadFile(f)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) != 4 {
		return 0, fmt.Errorf("content of %s is not compliant with SMB mpi_overhead format (%d lines instead of 4)", f, len(lines))
	}
	targetLine := lines[2]
	for {
		// Cleanup the line to make parsing more reliable
		if !strings.Contains(targetLine, "  ") {
			break
		}
		targetLine = strings.ReplaceAll(targetLine, "  ", " ")
	}
	words := strings.Split(targetLine, " ")
	if len(words) != 8 {
		return 0, fmt.Errorf("invalid format: %s, %d elements instead of 8: %s", lines[2], len(words), words)
	}
	value, err := strconv.ParseFloat(words[6], 32)
	if err != nil {
		return 0, fmt.Errorf("unable to get actual data from %s: %w", words[6], err)
	}
	return float32(value), nil
}

func GetBandwidth(d *Data) (float64, string, error) {
	if d == nil {
		return 0, "", fmt.Errorf("undefined data")
	}

	unit := "Unknown"
	header := d.Text[2]
	header = util.CleanOSUline(header)
	unit = strings.TrimPrefix(header, "# Size Bandwidth (")
	unit = strings.TrimSuffix(unit, ")")

	idx := len(d.Text) - 1
	for {
		if d.Text[idx] == "" {
			idx--
		} else {
			break
		}
	}
	if !strings.HasPrefix(d.Text[idx], "4194304") {
		return -1, unit, fmt.Errorf("unexpected data, unable to find result for 4M messages: %s (idx: %d)", d.Text[idx], idx)
	}
	bw := 0.0
	for i := 0; i <= 5; i++ {
		line := d.Text[idx-i]
		line = util.CleanOSUline(line)
		tokens := strings.Split(line, " ")
		if len(tokens) != 2 {
			return -1, unit, fmt.Errorf("unexpected data, unable to find size and data: %s (idx: %d, len: %d)", line, idx, len(tokens))
		}
		tokens[1] = strings.TrimLeft(tokens[1], " ")
		tokens[1] = strings.TrimLeft(tokens[1], "\t")
		val, err := strconv.ParseFloat(tokens[1], 32)
		if err != nil {
			return -1, unit, fmt.Errorf("unexpected data, unable to parse data: %s (idx: %d)", d.Text[idx], idx)
		}
		if bw < val {
			bw = val
		}
	}

	if unit == "MB/s" {
		bw = bw / 125
		unit = "Gb/s"
	}

	return bw, unit, nil
}

func ComputeOverlap(smbMPIOverhead float32, overlapData map[string][]string) (float32, map[string]float32, error) {
	numBenchs := len(overlapData)
	skipped := 0
	overlapDetails := make(map[string]float32)
	var finalOverlap float32
	finalOverlap = 0.0

	for benchName, output := range overlapData {
		if benchName == "overlap_ibarrier" {
			// We do not include data for ibarrier since it is difficult to have results for it
			// that makes statistical sense
			skipped++
			continue
		}
		overlapDetails[benchName] = 0.0
		for _, line := range output {
			if strings.HasPrefix(line, "Overlap: ") {
				line = strings.TrimPrefix(line, "Overlap: ")
				line = strings.TrimRight(line, "\n")
				line = strings.TrimSuffix(line, " %")
				value, err := strconv.ParseFloat(line, 32)
				if err != nil {
					return 0, nil, err
				}
				overlapDetails[benchName] = float32(value)
				finalOverlap += float32(value)
				break
			}
		}
	}
	if smbMPIOverhead < 0 {
		smbMPIOverhead = 0.0
	}
	finalOverlap += smbMPIOverhead
	overlapDetails["SMB mpi_overhead"] = smbMPIOverhead

	// Before we exit, we make sure we have all the expected data and if not, we add the
	// missing data as 0
	added := 0
	overlapBenchs := overlap.GetListSubBenchmarks()
	for _, subBench := range overlapBenchs {
		if _, ok := overlapDetails[subBench]; !ok {
			overlapDetails[subBench] = 0.0
			overlapData[subBench] = []string{"Data missing"}
			added++
		}
	}

	numBenchs -= skipped
	numBenchs += added
	return finalOverlap / float32(numBenchs+1), overlapDetails, nil
}

func Get(dir string) (*Results, error) {
	r := new(Results)
	r.osuResultFiles = make(map[string]string)
	r.smbResultFiles = make(map[string]string)
	r.overlapResultFiles = make(map[string]string)
	r.resultsDir = dir

	if r.resultsDir == "" {
		return nil, fmt.Errorf("undefined result directory")
	}

	labelFile := label.GetFilePath(r.resultsDir)
	labels := make(map[string]string)
	err := label.FromFile(labelFile, labels)
	if err != nil {
		return nil, fmt.Errorf("label.FromFile() failed: %w", err)
	}

	outputFiles, err := util.GetOutputFiles(dir)
	if err != nil {
		return nil, err
	}

	for hash, expLabel := range labels {
		filePath := outputFiles[hash]
		if filePath == "" {
			return nil, fmt.Errorf("unable to find output file for %s", expLabel)
		}

		if strings.HasPrefix(expLabel, "osu") {
			// OSU-type output file
			r.osuResultFiles[expLabel] = filePath
		}

		if strings.HasPrefix(expLabel, "smb") {
			// SMB-type output file
			r.smbResultFiles[expLabel] = filePath
		}

		if strings.HasPrefix(expLabel, "overlap") {
			// overlap-type output file
			expLabel = strings.ReplaceAll(expLabel, "overlap_overlap_", "overlap_")
			r.overlapResultFiles[expLabel] = filePath
		}
	}

	return r, nil
}

func ComputeScore(dataDir string) (*score.Metrics, error) {
	data, err := Get(dataDir)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, fmt.Errorf("unable to load results")
	}

	osuData := data.LoadResultsWithPrefix("osu")
	mpiOverhead, err := data.GetSMBOverlap()
	if err != nil {
		return nil, err
	}
	bwData := osuData["bw"]
	if bwData == nil {
		return nil, fmt.Errorf("bandwidth data is missing")
	}
	bandwidth, bandwidthUnit, err := GetBandwidth(bwData)
	if err != nil {
		return nil, err
	}
	latency, latencyUnit, err := data.GetLatency()
	if err != nil {
		return nil, err
	}

	metrics := new(score.Metrics)
	metrics.Bandwidth = bandwidth
	metrics.BandwidthUnit = bandwidthUnit
	metrics.Latency = float64(latency)
	metrics.LatencyUnit = latencyUnit
	metrics.MpiOverlap = float32(mpiOverhead)
	metrics.OverlapData = data.GetOverlapData()
	metrics.OverlapScore, metrics.OverlapDetails, err = ComputeOverlap(metrics.MpiOverlap, metrics.OverlapData)
	if err != nil {
		return nil, err
	}

	if bandwidthUnit != "Gb/s" {
		return nil, fmt.Errorf("unsupported unit for bandwidth (%s)", bandwidthUnit)
	}
	if latencyUnit != "us" {
		return nil, fmt.Errorf("unsupported unit for latency (%s)", latencyUnit)
	}

	metrics.Score = metrics.Compute()
	return metrics, nil
}

func String(dataDir string) (string, error) {
	metrics, err := ComputeScore(dataDir)
	if err != nil {
		return "", err
	}

	return metrics.ToString(), nil

}

func Create(outputDir string, dataDir string) error {
	filePath := filepath.Join(outputDir, FileName)
	content, err := String(dataDir)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filePath, []byte(content), FilePermission)
	if err != nil {
		return err
	}
	return nil
}

func Save(path string, s *score.Metrics) error {
	content := s.ToString()
	return ioutil.WriteFile(path, []byte(content), FilePermission)
}
