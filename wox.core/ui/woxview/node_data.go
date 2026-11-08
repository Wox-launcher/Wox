package woxview

import "fmt"

// NodeData replaces the supplied series fields of one keyed chart. Omitted
// fields retain the current data, and an empty array explicitly clears a field.
type NodeData struct {
	Values      []float64      `json:"values,omitempty"`
	PointLabels []string       `json:"pointLabels,omitempty"`
	Points      []ScatterPoint `json:"points,omitempty"`
	PointRadii  []float32      `json:"pointRadii,omitempty"`
}

// ParseNodeData bounds and decodes the data-only payload before entering the UI thread.
func ParseNodeData(data []byte) (NodeData, error) {
	var patch NodeData
	if len(data) > MaxBytes {
		return patch, fmt.Errorf("woxview node data exceeds %d bytes", MaxBytes)
	}
	if err := decodeStrict(data, &patch); err != nil {
		return patch, err
	}
	return patch, nil
}

// UpdateNodeData stages a complete replacement of the supplied data fields for
// the next frame, keeping the authored tree and the chart's retained state.
func (v *View) UpdateNodeData(key string, data NodeData) error {
	var target *nodeLifetime
	for _, owner := range v.nodes {
		if key != "" && owner.node.Key == key {
			if target != nil {
				return fmt.Errorf("woxview node key %q is ambiguous", key)
			}
			target = owner
		}
	}
	if target == nil {
		return fmt.Errorf("woxview node key %q was not found", key)
	}
	props := target.dataProps
	if props == nil {
		props = target.node.Props
	}
	switch current := props.(type) {
	case *LineChartProps:
		if (data.Values == nil && data.PointLabels == nil) || data.Points != nil || data.PointRadii != nil {
			return fmt.Errorf("line-chart data accepts values and pointLabels")
		}
		next := *current
		if data.Values != nil {
			next.Values = data.Values
		}
		if data.PointLabels != nil {
			next.PointLabels = data.PointLabels
		}
		if err := validateChartValues(NodeLineChart, next.Values, next.PointLabels, 4096); err != nil {
			return err
		}
		target.dataProps = &next
	case *ScatterChartProps:
		if (data.Points == nil && data.PointRadii == nil) || data.Values != nil || data.PointLabels != nil {
			return fmt.Errorf("scatter-chart data accepts points and pointRadii")
		}
		next := *current
		if data.Points != nil {
			next.Points = data.Points
		}
		if data.PointRadii != nil {
			next.PointRadii = data.PointRadii
		}
		if data.Points != nil {
			if err := next.prepare(); err != nil {
				return err
			}
		} else if err := validateScatterRadii(next.PointRadii, len(next.Points)); err != nil {
			return err
		}
		target.dataProps = &next
	default:
		return fmt.Errorf("%s does not support woxview node data updates", target.node.Type)
	}
	// Virtualized nodes keep the snapshot and pick it up when mounted again.
	target.dataContext.Invalidate()
	return nil
}
