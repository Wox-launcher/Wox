package ui

import (
	"encoding/json"
	"net/http"
	"strings"

	"wox/util/tooltip"
)

type tooltipOverlayRequest struct {
	Name         string  `json:"name"`
	Text         string  `json:"text"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	AnchorX      float64 `json:"anchorX"`
	AnchorY      float64 `json:"anchorY"`
	AnchorWidth  float64 `json:"anchorWidth"`
	AnchorHeight float64 `json:"anchorHeight"`
}

func handleTooltipOverlayShow(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	var request tooltipOverlayRequest
	if !readJSONRequest(w, r, &request, "tooltip overlay request") {
		return
	}

	request.Name = strings.TrimSpace(request.Name)
	request.Text = strings.TrimSpace(request.Text)
	if request.Name == "" || request.Text == "" {
		writeErrorResponse(w, "tooltip name and text are required")
		return
	}

	tooltip.Show(ctx, tooltip.OverlayOptions{
		Name:         request.Name,
		Text:         request.Text,
		X:            request.X,
		Y:            request.Y,
		AnchorX:      request.AnchorX,
		AnchorY:      request.AnchorY,
		AnchorWidth:  request.AnchorWidth,
		AnchorHeight: request.AnchorHeight,
	})

	writeSuccessResponse(w, "")
}

func handleTooltipOverlayHide(w http.ResponseWriter, r *http.Request) {
	var request tooltipOverlayRequest
	if !readJSONRequest(w, r, &request, "tooltip overlay request") {
		return
	}

	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		writeErrorResponse(w, "tooltip name is required")
		return
	}

	tooltip.Close(request.Name)
	writeSuccessResponse(w, "")
}

func readJSONRequest(w http.ResponseWriter, r *http.Request, target any, label string) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeErrorResponse(w, "failed to parse "+label+": "+err.Error())
		return false
	}
	return true
}
