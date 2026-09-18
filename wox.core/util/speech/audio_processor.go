package speech

import (
	"math"
	"time"
)

const (
	audioSampleRate       = 16000
	initialNoiseFloorDBFS = -60.0
	voiceMarginDB         = 6.0
	noiseFloorQuietTime   = time.Second
	noiseFloorActiveTime  = 30 * time.Second
	previewPeakLimit      = 0.99
	previewRMSTarget      = 0.40
)

// AudioProcessingStats describes cumulative level measurements for a session.
type AudioProcessingStats struct {
	InputRMSDBFS      float64
	InputPeakDBFS     float64
	OutputRMSDBFS     float64
	OutputPeakDBFS    float64
	NoiseFloorDBFS    float64
	AverageGainDB     float64
	MaximumGainDB     float64
	CandidateDuration time.Duration
}

type audioCandidateRange struct {
	StartSample int
	EndSample   int
}

// AdaptiveAudioProcessor watches raw microphone levels for offline fallback.
// It never rewrites recognition audio.
type AdaptiveAudioProcessor struct {
	candidateFloorDBFS float64

	inputSquares float64
	inputPeak    float64
	totalSamples int64

	processedSamples int
	candidateSamples int
	candidateRanges  []audioCandidateRange
}

// NewAdaptiveAudioProcessor creates a processor with a conservative initial noise floor.
func NewAdaptiveAudioProcessor() *AdaptiveAudioProcessor {
	return &AdaptiveAudioProcessor{candidateFloorDBFS: initialNoiseFloorDBFS}
}

// applyPreviewVolume raises a copy of the waveform so preview playback is
// audible. Isolated peaks may clip so quiet speech reaches the RMS target.
func applyPreviewVolume(samples []float32) ([]float32, float64) {
	out := append([]float32(nil), samples...)
	peak := waveformPeak(out)
	if peak <= 0 {
		return out, 0
	}
	gain := previewPeakLimit / peak
	if rms := waveformRMS(out); rms > 0 {
		if rmsGain := previewRMSTarget / rms; rmsGain > gain {
			gain = rmsGain
		}
	}
	if gain <= 1 {
		return out, 0
	}
	for i, sample := range out {
		value := float64(sample) * gain
		if value > previewPeakLimit {
			value = previewPeakLimit
		} else if value < -previewPeakLimit {
			value = -previewPeakLimit
		}
		out[i] = float32(value)
	}
	return out, 20 * math.Log10(gain)
}

// Observe records raw levels and speech-like ranges without changing samples.
func (p *AdaptiveAudioProcessor) Observe(samples []float32) {
	if len(samples) == 0 {
		return
	}

	var inputSquares float64
	var inputPeak float64
	for _, sample := range samples {
		value := math.Abs(float64(sample))
		inputSquares += value * value
		if value > inputPeak {
			inputPeak = value
		}
	}

	inputRMSDBFS := amplitudeToDBFS(math.Sqrt(inputSquares / float64(len(samples))))
	isCandidate := inputRMSDBFS >= p.candidateFloorDBFS+voiceMarginDB
	chunkDuration := time.Duration(float64(time.Second) * float64(len(samples)) / audioSampleRate)
	p.updateCandidateFloor(isCandidate, inputRMSDBFS, chunkDuration)
	p.trackCandidateRange(isCandidate, len(samples))

	p.inputSquares += inputSquares
	p.inputPeak = math.Max(p.inputPeak, inputPeak)
	p.totalSamples += int64(len(samples))
	p.processedSamples += len(samples)
}

// Stats returns cumulative microphone measurements for logging and diagnostics.
func (p *AdaptiveAudioProcessor) Stats() AudioProcessingStats {
	stats := AudioProcessingStats{
		InputRMSDBFS:      -120,
		InputPeakDBFS:     -120,
		OutputRMSDBFS:     -120,
		OutputPeakDBFS:    -120,
		NoiseFloorDBFS:    p.candidateFloorDBFS,
		CandidateDuration: time.Duration(float64(time.Second) * float64(p.candidateSamples) / audioSampleRate),
	}
	if p.totalSamples == 0 {
		return stats
	}
	stats.InputRMSDBFS = amplitudeToDBFS(math.Sqrt(p.inputSquares / float64(p.totalSamples)))
	stats.InputPeakDBFS = amplitudeToDBFS(p.inputPeak)
	return stats
}

// CandidateRanges returns speech-like intervals using raw sample offsets.
func (p *AdaptiveAudioProcessor) CandidateRanges() []audioCandidateRange {
	ranges := make([]audioCandidateRange, len(p.candidateRanges))
	copy(ranges, p.candidateRanges)
	return ranges
}

func (p *AdaptiveAudioProcessor) updateCandidateFloor(isCandidate bool, inputRMSDBFS float64, chunkDuration time.Duration) {
	noiseTime := noiseFloorActiveTime
	if !isCandidate {
		noiseTime = noiseFloorQuietTime
	}
	p.candidateFloorDBFS += smoothingAlpha(chunkDuration, noiseTime) * (inputRMSDBFS - p.candidateFloorDBFS)
}

func (p *AdaptiveAudioProcessor) trackCandidateRange(candidate bool, sampleCount int) {
	start := p.processedSamples
	end := start + sampleCount
	if !candidate {
		return
	}
	p.candidateSamples += sampleCount
	if len(p.candidateRanges) > 0 && p.candidateRanges[len(p.candidateRanges)-1].EndSample == start {
		p.candidateRanges[len(p.candidateRanges)-1].EndSample = end
		return
	}
	p.candidateRanges = append(p.candidateRanges, audioCandidateRange{StartSample: start, EndSample: end})
}

// previewVolumeStats fills dump metadata for the constant preview gain.
func previewVolumeStats(stats AudioProcessingStats, boosted []float32, gainDB float64) AudioProcessingStats {
	stats.AverageGainDB = gainDB
	stats.MaximumGainDB = gainDB
	if len(boosted) == 0 {
		return stats
	}
	stats.OutputRMSDBFS = amplitudeToDBFS(waveformRMS(boosted))
	stats.OutputPeakDBFS = amplitudeToDBFS(waveformPeak(boosted))
	return stats
}

func waveformPeak(samples []float32) float64 {
	var peak float64
	for _, sample := range samples {
		value := math.Abs(float64(sample))
		if value > peak {
			peak = value
		}
	}
	return peak
}

func waveformRMS(samples []float32) float64 {
	if len(samples) == 0 {
		return 0
	}
	var squares float64
	for _, sample := range samples {
		value := float64(sample)
		squares += value * value
	}
	return math.Sqrt(squares / float64(len(samples)))
}

func smoothingAlpha(duration time.Duration, timeConstant time.Duration) float64 {
	if duration <= 0 || timeConstant <= 0 {
		return 1
	}
	return 1 - math.Exp(-float64(duration)/float64(timeConstant))
}

func amplitudeToDBFS(amplitude float64) float64 {
	if amplitude <= 0 {
		return -120
	}
	return math.Max(-120, 20*math.Log10(amplitude))
}
