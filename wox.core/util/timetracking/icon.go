package timetracking

// IconConversionDiagnostics identifies a query-scoped icon conversion in timing logs.
type IconConversionDiagnostics struct {
	Purpose     string
	QueryId     string
	Plugin      string
	ResultId    string
	ResultTitle string
	// Recorder collects timing in the caller when per-conversion log lines would distort hot-path measurements.
	Recorder func(IconConversionDiagnostics, IconConversionTimingSummary)
}

// Enabled reports whether the caller requested icon conversion diagnostics.
func (d IconConversionDiagnostics) Enabled() bool {
	return d.Recorder != nil || d.Purpose != "" || d.QueryId != "" || d.Plugin != "" || d.ResultId != ""
}

// IconConversionTimingSummary exposes query-scoped icon conversion timing without forcing each conversion to write its own log line.
type IconConversionTimingSummary struct {
	TotalCost       int64
	TotalCostUs     int64
	FileIconCost    int64
	FileIconCostUs  int64
	RelativeCost    int64
	RelativeCostUs  int64
	SvgCheckCost    int64
	SvgCheckCostUs  int64
	CacheCost       int64
	CacheCostUs     int64
	CacheHit        bool
	CacheSource     string
	LazyCheckCost   int64
	LazyCheckCostUs int64
	Lazy            bool
	LazyReason      string
	LazyWidth       int
	LazyHeight      int
	CropCost        int64
	CropCostUs      int64
	CropResult      string
	CropCache       string
	CropMetadataMs  int64
	CropDecodeMs    int64
	CropScanMs      int64
	CropSaveMs      int64
	ResizeCost      int64
	ResizeCostUs    int64
	ResizeResult    string
	ResizeCache     string
	ResizeDecodeMs  int64
	ResizeOpMs      int64
	ResizeSaveMs    int64
	ResizeSourceW   int
	ResizeSourceH   int
	ResizeTarget    int
	NormalizedType  string
	OutputType      string
	OutputDataLen   int
}

// IconConversionTiming keeps detailed icon conversion timing while common image
// code samples each step and decides whether to log or aggregate it.
type IconConversionTiming struct {
	TotalCost       int64
	TotalCostUs     int64
	FileIconCost    int64
	FileIconCostUs  int64
	RelativeCost    int64
	RelativeCostUs  int64
	SvgCheckCost    int64
	SvgCheckCostUs  int64
	CacheCost       int64
	CacheCostUs     int64
	CacheHit        bool
	CacheSource     string
	LazyCheckCost   int64
	LazyCheckCostUs int64
	Lazy            bool
	LazyReason      string
	LazyWidth       int
	LazyHeight      int
	CropCost        int64
	CropCostUs      int64
	CropTiming      IconCropTiming
	ResizeCost      int64
	ResizeCostUs    int64
	ResizeTiming    IconResizeTiming
	NormalizedType  string
	OutputType      string
	OutputDataLen   int
}

// Summary converts detailed icon conversion timing into the public aggregate DTO.
func (t IconConversionTiming) Summary() IconConversionTimingSummary {
	return IconConversionTimingSummary{
		TotalCost:       t.TotalCost,
		TotalCostUs:     t.TotalCostUs,
		FileIconCost:    t.FileIconCost,
		FileIconCostUs:  t.FileIconCostUs,
		RelativeCost:    t.RelativeCost,
		RelativeCostUs:  t.RelativeCostUs,
		SvgCheckCost:    t.SvgCheckCost,
		SvgCheckCostUs:  t.SvgCheckCostUs,
		CacheCost:       t.CacheCost,
		CacheCostUs:     t.CacheCostUs,
		CacheHit:        t.CacheHit,
		CacheSource:     t.CacheSource,
		LazyCheckCost:   t.LazyCheckCost,
		LazyCheckCostUs: t.LazyCheckCostUs,
		Lazy:            t.Lazy,
		LazyReason:      t.LazyReason,
		LazyWidth:       t.LazyWidth,
		LazyHeight:      t.LazyHeight,
		CropCost:        t.CropCost,
		CropCostUs:      t.CropCostUs,
		CropResult:      t.CropTiming.Result,
		CropCache:       t.CropTiming.CacheSource,
		CropMetadataMs:  t.CropTiming.MetadataMs,
		CropDecodeMs:    t.CropTiming.DecodeMs,
		CropScanMs:      t.CropTiming.CropMs,
		CropSaveMs:      t.CropTiming.SaveMs,
		ResizeCost:      t.ResizeCost,
		ResizeCostUs:    t.ResizeCostUs,
		ResizeResult:    t.ResizeTiming.Result,
		ResizeCache:     t.ResizeTiming.CacheSource,
		ResizeDecodeMs:  t.ResizeTiming.DecodeMs,
		ResizeOpMs:      t.ResizeTiming.ResizeMs,
		ResizeSaveMs:    t.ResizeTiming.SaveMs,
		ResizeSourceW:   t.ResizeTiming.SourceWidth,
		ResizeSourceH:   t.ResizeTiming.SourceHeight,
		ResizeTarget:    t.ResizeTiming.TargetSize,
		NormalizedType:  t.NormalizedType,
		OutputType:      t.OutputType,
		OutputDataLen:   t.OutputDataLen,
	}
}

// IconCropTiming records crop-cache and transparent-padding crop costs.
type IconCropTiming struct {
	CacheSource string
	MetadataMs  int64
	DecodeMs    int64
	CropMs      int64
	SaveMs      int64
	Result      string
}

// IconResizeTiming records resize-cache, decode, resize, and save costs.
type IconResizeTiming struct {
	CacheSource  string
	DecodeMs     int64
	ResizeMs     int64
	SaveMs       int64
	Result       string
	SourceWidth  int
	SourceHeight int
	TargetSize   int
}
