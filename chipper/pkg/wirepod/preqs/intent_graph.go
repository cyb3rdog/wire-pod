package processreqs

import (
	"strings"

	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/kercre123/wire-pod/chipper/pkg/vtt"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
	ttr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/ttr"
)

func (s *Server) ProcessIntentGraph(req *vtt.IntentGraphRequest) (*vtt.IntentGraphResponse, error) {
	speechReq := sr.ReqToSpeechRequest(req)
	transcribedText, err := sttHandler(speechReq)
	if err != nil {
		ttr.IntentPass(req, "intent_system_noaudio", "voice processing error: "+err.Error(), map[string]string{"error": err.Error()}, true)
		return nil, nil
	}
	if strings.TrimSpace(transcribedText) == "" {
		ttr.IntentPass(req, "intent_system_noaudio", "", map[string]string{}, false)
		return nil, nil
	}
	successMatched := ttr.ProcessTextAll(req, transcribedText, vars.IntentList, speechReq.IsOpus)
	if !successMatched {
		knowledge := vars.GetAPIConfig().Knowledge
		if knowledge.IntentGraph && knowledge.Enable {
			if knowledge.Provider == "houndify" {
				if len([]rune(transcribedText)) >= 8 {
					logger.Println("No intent matched, forwarding to Houndify for device " + req.Device + "...")
					InitKnowledge() // Errors without this for whatever reason even though I think it should be inited already
					apiResponse := houndifyTextRequest(transcribedText, req.Device, req.Session)
					if apiResponse != "" && !strings.Contains(apiResponse, "not enabled") && !strings.Contains(apiResponse, "Knowledge graph is not enabled") && !strings.Contains(apiResponse, "Didn't get that!") {
						ttr.KnowledgeGraphResponseIG(req, apiResponse, transcribedText)
						logger.Println("Bot " + speechReq.Device + " request served via Houndify.")
						return nil, nil
					}
					// If Houndify fails or returns nothing useful, fall through to unmatched
					logger.Println("Houndify returned empty or error response")
				}
			} else {
				logger.Println("Making LLM request for device " + req.Device + "...")
				_, err := ttr.StreamingKGSim(req, req.Device, transcribedText, false)
				if err != nil {
					logger.Println("LLM error: " + err.Error())
					logger.LogUI("LLM error: " + err.Error())
					ttr.IntentPass(req, "intent_system_unmatched", transcribedText, map[string]string{"": ""}, false)
					ttr.KGSim(req.Device, "There was an error getting a response from the L L M. Check the logs in the web interface.")
				}
				logger.Println("Bot " + speechReq.Device + " request served.")
				return nil, nil
			}
		}
		logger.Println("No intent was matched.")
		ttr.IntentPass(req, "intent_system_unmatched", transcribedText, map[string]string{"": ""}, false)
		return nil, nil
	}
	logger.Println("Bot " + speechReq.Device + " request served.")
	return nil, nil
}
