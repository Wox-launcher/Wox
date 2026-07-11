package speech

import (
	"math"
	"time"
)

const (
	audioSampleRate       = 16000
	initialNoiseFloorDBFS = -60.0
	voiceMarginDB         = 6.0
	targetQuietSpeechDBFS = -24.0
	maximumAdaptiveGainDB = 24.0
	processorPeakLimit    = 0.95
	processorSampleClamp  = 0.98
	noiseFloorQuietTime   = time.Second
	noiseFloorActiveTime  = 30 * time.Second
	gainRiseTime          = 250 * time.Millisecond
	gainFallTime          = 50 * time.Millisecond
)

// AudioProcessingStats describes cumulative level and gain measurements for a session.
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

// AdaptiveAudioProcessor raises quiet speech while preserving normal-volume input.
type AdaptiveAudioProcessor struct {
	noiseFloorDBFS float64
	currentGainDB  float64

	inputSquares  float64
	outputSquares float64
	inputPeak     float64
	outputPeak    float64
	gainDBSamples float64
	maximumGainDB float64
	totalSamples  int64

	processedSamples int
	candidateSamples int
	candidateRanges  []audioCandidateRange
}

// NewAdaptiveAudioProcessor creates a processor with a conservative initial noise floor.
func NewAdaptiveAudioProcessor() *AdaptiveAudioProcessor {
	return &AdaptiveAudioProcessor{noiseFloorDBFS: initialNoiseFloorDBFS}
}

// Process enhances samples in place and updates session-level measurements.
func (p *AdaptiveAudioProcessor) Process(samples []float32) {
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

	inputRMS := math.Sqrt(inputSquares / float64(len(samples)))
	inputRMSDBFS := amplitudeToDBFS(inputRMS)
	noiseBeforeUpdate := p.noiseFloorDBFS
	isCandidate := inputRMSDBFS >= noiseBeforeUpdate+voiceMarginDB
	chunkDuration := time.Duration(float64(time.Second) * float64(len(samples)) / audioSampleRate)

	noiseTime := noiseFloorActiveTime
	if !isCandidate {
		noiseTime = noiseFloorQuietTime
	}
	noiseAlpha := smoothingAlpha(chunkDuration, noiseTime)
	p.noiseFloorDBFS += noiseAlpha * (inputRMSDBFS - p.noiseFloorDBFS)
	p.trackCandidateRange(isCandidate, len(samples))

	desiredGainDB := 0.0
	if isCandidate && inputRMSDBFS < targetQuietSpeechDBFS {
		desiredGainDB = math.Min(targetQuietSpeechDBFS-inputRMSDBFS, maximumAdaptiveGainDB)
	}
	if inputPeak > 0 {
		peakSafeGainDB := 20 * math.Log10(processorPeakLimit/inputPeak)
		desiredGainDB = math.Min(desiredGainDB, math.Max(0, peakSafeGainDB))
	}

	startGainDB := p.currentGainDB
	gainTime := gainFallTime
	if desiredGainDB > startGainDB {
		gainTime = gainRiseTime
	}
	p.currentGainDB += smoothingAlpha(chunkDuration, gainTime) * (desiredGainDB - p.currentGainDB)
	endGainDB := p.currentGainDB

	var outputSquares float64
	var outputPeak float64
	for i, sample := range samples {
		progress := float64(i+1) / float64(len(samples))
		gainDB := startGainDB + (endGainDB-startGainDB)*progress
		value := float64(sample) * math.Pow(10, gainDB/20)
		if value > processorPeakLimit {
			value = processorPeakLimit
		} else if value < -processorPeakLimit {
			value = -processorPeakLimit
		}
		value = math.Max(-processorSampleClamp, math.Min(processorSampleClamp, value))
		samples[i] = float32(value)
		absolute := math.Abs(value)
		outputSquares += absolute * absolute
		if absolute > outputPeak {
			outputPeak = absolute
		}
	}

	p.inputSquares += inputSquares
	p.outputSquares += outputSquares
	p.inputPeak = math.Max(p.inputPeak, inputPeak)
	p.outputPeak = math.Max(p.outputPeak, outputPeak)
	p.gainDBSamples += ((startGainDB + endGainDB) / 2) * float64(len(samples))
	p.maximumGainDB = math.Max(p.maximumGainDB, math.Max(startGainDB, endGainDB))
	p.totalSamples += int64(len(samples))
	p.processedSamples += len(samples)
}

// Stats returns cumulative processor measurements for logging and diagnostics.
func (p *AdaptiveAudioProcessor) Stats() AudioProcessingStats {
	stats := AudioProcessingStats{
		InputRMSDBFS:      -120,
		InputPeakDBFS:     -120,
		OutputRMSDBFS:     -120,
		OutputPeakDBFS:    -120,
		NoiseFloorDBFS:    p.noiseFloorDBFS,
		MaximumGainDB:     p.maximumGainDB,
		CandidateDuration: time.Duration(float64(time.Second) * float64(p.candidateSamples) / audioSampleRate),
	}
	if p.totalSamples == 0 {
		return stats
	}
	stats.InputRMSDBFS = amplitudeToDBFS(math.Sqrt(p.inputSquares / float64(p.totalSamples)))
	stats.InputPeakDBFS = amplitudeToDBFS(p.inputPeak)
	stats.OutputRMSDBFS = amplitudeToDBFS(math.Sqrt(p.outputSquares / float64(p.totalSamples)))
	stats.OutputPeakDBFS = amplitudeToDBFS(p.outputPeak)
	stats.AverageGainDB = p.gainDBSamples / float64(p.totalSamples)
	return stats
}

// CandidateRanges returns speech-like intervals using processed sample offsets.
func (p *AdaptiveAudioProcessor) CandidateRanges() []audioCandidateRange {
	ranges := make([]audioCandidateRange, len(p.candidateRanges))
	copy(ranges, p.candidateRanges)
	return ranges
}

// trackCandidateRange merges adjacent candidate chunks into stable fallback regions.
func (p *AdaptiveAudioProcessor) trackCandidateRange(candidate bool, sampleCount int) {
	start := p.processedSamples
	end := start + sampleCount
	if candidate {
		p.candidateSamples += sampleCount
		if len(p.candidateRanges) > 0 && p.candidateRanges[len(p.candidateRanges)-1].EndSample == start {
			p.candidateRanges[len(p.candidateRanges)-1].EndSample = end
			return
		}
		p.candidateRanges = append(p.candidateRanges, audioCandidateRange{StartSample: start, EndSample: end})
	}
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
