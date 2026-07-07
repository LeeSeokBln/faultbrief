// Package baseline computes analysis/baseline windows and accumulates
// per-template bucketed counts used by the spike and novelty detectors.
package baseline

import (
	"strings"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
	"github.com/LeeSeokBln/faultbrief/internal/template"
)

// DefaultBucket is the histogram resolution for spike detection.
const DefaultBucket = 5 * time.Minute

// Windows holds the analysis window and the baseline window preceding it.
type Windows struct {
	AnalysisFrom, AnalysisTo time.Time
	BaselineFrom, BaselineTo time.Time
}

// Compute derives windows from now: analysis = [now-since, now-until),
// baseline = the span of baselineSpan directly before it.
func Compute(now time.Time, since, until, baselineSpan time.Duration) Windows {
	aTo := now.Add(-until)
	aFrom := now.Add(-since)
	return Windows{
		AnalysisFrom: aFrom, AnalysisTo: aTo,
		BaselineFrom: aFrom.Add(-baselineSpan), BaselineTo: aFrom,
	}
}

// TemplateStats aggregates one masked template inside the analysis window.
type TemplateStats struct {
	ID       string
	Masked   string
	Source   string
	Unit     string
	Count    int
	Buckets  []int
	Severity model.Severity // max severity seen
	Samples  []string       // up to 3 raw messages
	FirstTS  time.Time
	LastTS   time.Time
}

// Accumulator routes records into analysis/baseline aggregates in one pass.
type Accumulator struct {
	W      Windows
	Bucket time.Duration

	Analysis        map[string]*TemplateStats
	BaselineCounts  map[string]int
	BaselineBuckets map[string][]int

	// Error-severity aggregate (SevError and above), per bucket.
	ErrA, ErrB []int
	// nginx-access request/5xx aggregates, per bucket.
	ReqA, ReqB       []int
	Err5xxA, Err5xxB []int

	numA, numB int
}

func NewAccumulator(w Windows, bucket time.Duration) *Accumulator {
	numA := bucketCount(w.AnalysisFrom, w.AnalysisTo, bucket)
	numB := bucketCount(w.BaselineFrom, w.BaselineTo, bucket)
	return &Accumulator{
		W: w, Bucket: bucket,
		Analysis:        map[string]*TemplateStats{},
		BaselineCounts:  map[string]int{},
		BaselineBuckets: map[string][]int{},
		ErrA:            make([]int, numA), ErrB: make([]int, numB),
		ReqA: make([]int, numA), ReqB: make([]int, numB),
		Err5xxA: make([]int, numA), Err5xxB: make([]int, numB),
		numA: numA, numB: numB,
	}
}

func bucketCount(from, to time.Time, bucket time.Duration) int {
	n := int(to.Sub(from) / bucket)
	if to.Sub(from)%bucket != 0 {
		n++
	}
	if n < 1 {
		n = 1
	}
	return n
}

// NumAnalysisBuckets exposes the analysis bucket count for detectors.
func (a *Accumulator) NumAnalysisBuckets() int { return a.numA }

// NumBaselineBuckets exposes the baseline bucket count for detectors.
func (a *Accumulator) NumBaselineBuckets() int { return a.numB }

// Add routes one record to the right window. Records outside both windows
// are ignored.
func (a *Accumulator) Add(rec model.LogRecord) {
	inA := !rec.TS.Before(a.W.AnalysisFrom) && rec.TS.Before(a.W.AnalysisTo)
	inB := !inA && !rec.TS.Before(a.W.BaselineFrom) && rec.TS.Before(a.W.BaselineTo)
	if !inA && !inB {
		return
	}
	masked := template.Mask(rec.Message)
	id := template.Fingerprint(rec.Source, masked)

	if inA {
		idx := a.bucketIdx(rec.TS, a.W.AnalysisFrom, a.numA)
		st, ok := a.Analysis[id]
		if !ok {
			st = &TemplateStats{
				ID: id, Masked: masked, Source: rec.Source, Unit: rec.Unit,
				Buckets: make([]int, a.numA), Severity: rec.Severity,
				FirstTS: rec.TS, LastTS: rec.TS,
			}
			a.Analysis[id] = st
		}
		st.Count++
		st.Buckets[idx]++
		if rec.Severity > st.Severity {
			st.Severity = rec.Severity
		}
		if rec.TS.Before(st.FirstTS) {
			st.FirstTS = rec.TS
		}
		if rec.TS.After(st.LastTS) {
			st.LastTS = rec.TS
		}
		if len(st.Samples) < 3 {
			st.Samples = append(st.Samples, rec.Message)
		}
		a.aggregate(rec, idx, a.ErrA, a.ReqA, a.Err5xxA)
		return
	}

	idx := a.bucketIdx(rec.TS, a.W.BaselineFrom, a.numB)
	a.BaselineCounts[id]++
	bb, ok := a.BaselineBuckets[id]
	if !ok {
		bb = make([]int, a.numB)
		a.BaselineBuckets[id] = bb
	}
	bb[idx]++
	a.aggregate(rec, idx, a.ErrB, a.ReqB, a.Err5xxB)
}

func (a *Accumulator) aggregate(rec model.LogRecord, idx int, errB, reqB, e5xxB []int) {
	if rec.Severity >= model.SevError {
		errB[idx]++
	}
	if rec.Source == "nginx-access" {
		reqB[idx]++
		if s := rec.Fields["status"]; len(s) == 3 && strings.HasPrefix(s, "5") {
			e5xxB[idx]++
		}
	}
}

func (a *Accumulator) bucketIdx(ts, from time.Time, n int) int {
	idx := int(ts.Sub(from) / a.Bucket)
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return idx
}
