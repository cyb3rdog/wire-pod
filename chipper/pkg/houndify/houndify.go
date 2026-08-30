// Package houndify holds parsing logic for the Houndify API's response
// format, shared between two independent integrations that both happen
// to target Houndify: the STT backend (pkg/wirepod/stt/houndify) and the
// Knowledge Graph "Ask" provider (pkg/wirepod/preqs). Neither is a
// natural home for the other to import -- an STT backend is meant to be
// a leaf dependency (see stt/dispatch's doc comment), and preqs's
// Houndify usage is a distinct feature (a knowledge-graph provider, not
// speech-to-text) that happens to share nothing with the STT backend
// beyond this one response shape. A small top-level shared package,
// matching this repo's existing pattern for cross-cutting utilities
// (fileutil, logger, vtt), avoids coupling the two together.
package houndify

import (
	"encoding/json"
	"strings"

	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/pkg/errors"
)

// ParseSpokenResponse extracts the spoken-response text from a raw
// Houndify API JSON response.
func ParseSpokenResponse(serverResponseJSON string) (string, error) {
	result := make(map[string]interface{})
	err := json.Unmarshal([]byte(serverResponseJSON), &result)
	if err != nil {
		logger.Println(err.Error())
		return "", errors.New("failed to decode json")
	}

	status, ok := result["Status"].(string)
	if !ok {
		return "", errors.New("unexpected houndify response: missing Status")
	}
	if !strings.EqualFold(status, "OK") {
		if msg, ok := result["ErrorMessage"].(string); ok {
			return "", errors.New(msg)
		}
		return "", errors.New("houndify request failed")
	}

	numToReturn, ok := result["NumToReturn"].(float64)
	if !ok || numToReturn < 1 {
		return "", errors.New("no results to return")
	}

	allResults, ok := result["AllResults"].([]interface{})
	if !ok || len(allResults) == 0 {
		return "", errors.New("unexpected houndify response: missing AllResults")
	}
	firstResult, ok := allResults[0].(map[string]interface{})
	if !ok {
		return "", errors.New("unexpected houndify response: malformed result")
	}
	spokenResponse, ok := firstResult["SpokenResponseLong"].(string)
	if !ok {
		return "", errors.New("unexpected houndify response: missing SpokenResponseLong")
	}
	return spokenResponse, nil
}
