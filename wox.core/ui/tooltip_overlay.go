package ui

import (
	"encoding/json"
	"net/http"

	"wox/ui/contract"
)

type tooltipOverlayRequest struct {
	Name         string  `json:"name"`
	Text         string  `json:"text"`
	Side         string  `json:"side"`
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

	if err := NewCoreServices().ShowTooltip(ctx, getSessionIdFromHeader(r), contract.TooltipOptions{
		Name:         request.Name,
		Text:         request.Text,
		Side:         request.Side,
		AnchorX:      request.AnchorX,
		AnchorY:      request.AnchorY,
		AnchorWidth:  request.AnchorWidth,
		AnchorHeight: request.AnchorHeight,
	}); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleTooltipOverlayHide(w http.ResponseWriter, r *http.Request) {
	var request tooltipOverlayRequest
	if !readJSONRequest(w, r, &request, "tooltip overlay request") {
		return
	}

	if err := NewCoreServices().HideTooltip(getTraceContext(r), getSessionIdFromHeader(r), request.Name); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func readJSONRequest(w http.ResponseWriter, r *http.Request, target any, label string) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeErrorResponse(w, "failed to parse "+label+": "+err.Error())
		return false
	}
	return true
}
