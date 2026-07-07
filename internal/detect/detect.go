// Package detect turns accumulated log data into findings via three layers:
// curated signatures, statistical spikes, and novel-template detection.
package detect

import "github.com/LeeSeokBln/faultbrief/internal/model"

// Params tunes the statistical detectors. Zero value is not usable; call
// DefaultParams.
type Params struct {
	ZThreshold     float64        // spike: min z-score
	MinCount       int            // spike: min occurrences inside analysis window
	MinRatio       float64        // spike: min rate ratio vs baseline
	SpikeMinSev    model.Severity // spike: min template severity to consider
	NoveltyMin     int            // novelty: min occurrences to report
	MaxNovelty     int            // novelty: cap on reported templates
	Nginx5xxMinReq int            // 5xx-rate: min requests in window to judge
	Nginx5xxRate   float64        // 5xx-rate: min error rate
}

func DefaultParams() Params {
	return Params{
		ZThreshold: 3.0,
		MinCount:   10,
		MinRatio:   3.0,
		// Notice keeps 4xx floods and warning-level bursts, but stops
		// healthy-traffic surges (info-level 2xx templates) from paging.
		SpikeMinSev:    model.SevNotice,
		NoveltyMin:     3,
		MaxNovelty:     10,
		Nginx5xxMinReq: 50,
		Nginx5xxRate:   0.05,
	}
}
