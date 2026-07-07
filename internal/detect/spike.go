package detect

import (
	"fmt"
	"math"
	"sort"

	"github.com/LeeSeokBln/faultbrief/internal/baseline"
	"github.com/LeeSeokBln/faultbrief/internal/model"
)

// minStdDev floors sigma so that near-constant baselines still produce a
// finite, meaningful z-score.
const minStdDev = 0.5

// Spikes finds frequency anomalies: per-template spikes, the aggregate
// error-rate spike, and the nginx 5xx-rate check.
func Spikes(acc *baseline.Accumulator, p Params) []model.Finding {
	var fs []model.Finding

	// Per-template spikes. Sort ids for deterministic output.
	ids := make([]string, 0, len(acc.Analysis))
	for id := range acc.Analysis {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		st := acc.Analysis[id]
		if st.Count < p.MinCount {
			continue
		}
		bb, ok := acc.BaselineBuckets[id]
		if !ok {
			continue // absent from baseline: novelty detector's job
		}
		z, ratio, aMean, bMean := zScore(st.Count, acc.NumAnalysisBuckets(), bb)
		if z < p.ZThreshold || ratio < p.MinRatio {
			continue
		}
		sev := st.Severity
		if sev < model.SevWarning {
			sev = model.SevWarning
		}
		fs = append(fs, model.Finding{
			Kind:     model.KindSpike,
			RuleID:   id,
			Severity: sev,
			Title:    truncate(st.Masked, 100),
			Detail: fmt.Sprintf("%d occurrences (%.1f/bucket) vs baseline %.2f/bucket — z=%.1f, ×%.1f",
				st.Count, aMean, bMean, z, ratio),
			Count:   st.Count,
			Score:   z,
			Source:  st.Source,
			Unit:    st.Unit,
			Samples: st.Samples,
			FirstTS: st.FirstTS,
			LastTS:  st.LastTS,
		})
	}

	// Aggregate error-rate spike across all sources.
	aErr := sum(acc.ErrA)
	if aErr >= p.MinCount {
		z, ratio, aMean, bMean := zScore(aErr, acc.NumAnalysisBuckets(), acc.ErrB)
		if z >= p.ZThreshold && ratio >= p.MinRatio {
			fs = append(fs, model.Finding{
				Kind:     model.KindSpike,
				RuleID:   "error-rate",
				Severity: model.SevWarning,
				Title:    "Overall error rate spiked",
				Detail: fmt.Sprintf("%d error-level records (%.1f/bucket) vs baseline %.2f/bucket — z=%.1f, ×%.1f",
					aErr, aMean, bMean, z, ratio),
				Count:   aErr,
				Score:   z,
				Source:  "all",
				FirstTS: acc.W.AnalysisFrom,
				LastTS:  acc.W.AnalysisTo,
			})
		}
	}

	// nginx 5xx rate: absolute threshold, hardened by baseline comparison
	// when the baseline carries enough traffic.
	aReq, a5xx := sum(acc.ReqA), sum(acc.Err5xxA)
	bReq, b5xx := sum(acc.ReqB), sum(acc.Err5xxB)
	if aReq >= p.Nginx5xxMinReq {
		rate := float64(a5xx) / float64(aReq)
		baselineOK := bReq >= p.Nginx5xxMinReq
		bRate := 0.0
		if baselineOK {
			bRate = float64(b5xx) / float64(bReq)
		}
		if rate >= p.Nginx5xxRate && (!baselineOK || bRate == 0 || rate >= 3*bRate) {
			sev := model.SevError
			if rate >= 0.25 {
				sev = model.SevCritical
			}
			fs = append(fs, model.Finding{
				Kind:     model.KindSpike,
				RuleID:   "nginx-5xx-rate",
				Severity: sev,
				Title:    "nginx 5xx error rate is high",
				Detail: fmt.Sprintf("%d of %d requests failed (%.1f%%), baseline %.1f%%",
					a5xx, aReq, rate*100, bRate*100),
				Count:   a5xx,
				Score:   rate * 100,
				Source:  "nginx-access",
				Unit:    "nginx",
				FirstTS: acc.W.AnalysisFrom,
				LastTS:  acc.W.AnalysisTo,
			})
		}
	}
	return fs
}

// zScore compares the analysis per-bucket mean against the baseline bucket
// distribution. Returns z, rate ratio, and both means.
func zScore(analysisTotal, analysisBuckets int, baselineBuckets []int) (z, ratio, aMean, bMean float64) {
	aMean = float64(analysisTotal) / float64(analysisBuckets)
	bMean, bStd := meanStd(baselineBuckets)
	sigma := math.Max(bStd, minStdDev)
	z = (aMean - bMean) / sigma
	ratio = aMean / math.Max(bMean, 0.05)
	return z, ratio, aMean, bMean
}

func meanStd(xs []int) (mean, std float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sumV float64
	for _, x := range xs {
		sumV += float64(x)
	}
	mean = sumV / float64(len(xs))
	var sq float64
	for _, x := range xs {
		d := float64(x) - mean
		sq += d * d
	}
	std = math.Sqrt(sq / float64(len(xs)))
	return mean, std
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

// truncate is rune-safe: byte slicing would split multi-byte UTF-8
// characters (log messages are frequently non-ASCII).
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
