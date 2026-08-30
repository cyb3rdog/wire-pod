package processreqs

import (
	"regexp"
	"time"

	pb "github.com/digital-dream-labs/api/go/chipperpb"
	"github.com/kercre123/wire-pod/chipper/pkg/houndify"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/kercre123/wire-pod/chipper/pkg/vtt"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
	ttr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/ttr"
	houndifysdk "github.com/soundhound/houndify-sdk-go"
)

var HKGclient houndifysdk.Client
var HoundEnable bool = true

func InitKnowledge() {
	knowledge := vars.GetAPIConfig().Knowledge
	if knowledge.Enable && knowledge.Provider == "houndify" {
		if knowledge.ID == "" || knowledge.Key == "" {
			// In-memory only, like the direct field assignment this
			// replaces: InitKnowledge runs on every request (see
			// ProcessKnowledgeGraph), so persisting here would mean
			// writing to disk on every single knowledge-graph request
			// while Houndify credentials are missing, not just flagging
			// it for this request.
			vars.SetAPIConfigInMemory(func(cfg *vars.Config) {
				cfg.Knowledge.Enable = false
			})
			logger.Println("Houndify Client Key or ID was empty, not initializing kg client")
		} else {
			HKGclient = houndifysdk.Client{
				ClientID:  knowledge.ID,
				ClientKey: knowledge.Key,
			}
			HKGclient.EnableConversationState()
			logger.Println("Initialized Houndify client")
		}
	}
}

var NoResult string = "NoResultCommand"
var NoResultSpoken string

func houndifyKG(req sr.SpeechRequest) string {
	var apiResponse string
	knowledge := vars.GetAPIConfig().Knowledge
	if knowledge.Enable && knowledge.Provider == "houndify" {
		logger.Println("Sending request to Houndify...")
		serverResponse := StreamAudioToHoundify(req, HKGclient)
		apiResponse, _ = houndify.ParseSpokenResponse(serverResponse)
		logger.Println("Houndify response: " + apiResponse)
	} else {
		apiResponse = "Houndify is not enabled."
		logger.Println("Houndify is not enabled.")
	}
	return apiResponse
}

func streamingKG(req *vtt.KnowledgeGraphRequest, speechReq sr.SpeechRequest) string {
	// have him start "thinking" right after the text is transcribed
	sttStart := time.Now()
	transcribedText, err := sttHandler(speechReq)
	sttElapsed := time.Since(sttStart).Round(time.Millisecond)
	if err != nil {
		logger.Println("(KG) transcription failed after " + sttElapsed.String() + ": " + err.Error())
		return "There was an error."
	}
	logger.Println("(KG) transcribed in " + sttElapsed.String() + ": " + transcribedText)
	kg := pb.KnowledgeGraphResponse{
		Session:     req.Session,
		DeviceId:    req.Device,
		CommandType: NoResult,
		SpokenText:  "bla bla bla bla bla bla bla bla bla bla",
	}
	req.Stream.Send(&kg)
	_, err = ttr.StreamingKGSim(req, req.Device, transcribedText, true)
	if err != nil {
		// ttr.StreamingKGSim already logs a detailed diagnostic (endpoint,
		// model, elapsed time, and the actual HTTP status/body for a
		// non-2xx response) via logLLMError before returning here.
		logger.Println("LLM error: " + err.Error())
	}
	logger.Println("(KG) Bot " + speechReq.Device + " request served.")
	return ""
}

// Takes a SpeechRequest, figures out knowledgegraph provider, makes request, returns API response
func KgRequest(req *vtt.KnowledgeGraphRequest, speechReq sr.SpeechRequest) string {
	knowledge := vars.GetAPIConfig().Knowledge
	if knowledge.Enable {
		if knowledge.Provider == "houndify" {
			return houndifyKG(speechReq)
		}
	}
	return "Knowledge graph is not enabled. This can be enabled in the web interface."
}

func (s *Server) ProcessKnowledgeGraph(req *vtt.KnowledgeGraphRequest) (*vtt.KnowledgeGraphResponse, error) {
	InitKnowledge()
	speechReq := sr.ReqToSpeechRequest(req)
	knowledge := vars.GetAPIConfig().Knowledge
	if knowledge.Enable && knowledge.Provider != "houndify" {
		streamingKG(req, speechReq)
	} else {
		apiResponse := KgRequest(req, speechReq)
		kg := pb.KnowledgeGraphResponse{
			Session:     req.Session,
			DeviceId:    req.Device,
			CommandType: NoResult,
			SpokenText:  apiResponse,
		}
		logger.Println("(KG) Bot " + speechReq.Device + " request served.")
		if err := req.Stream.Send(&kg); err != nil {
			return nil, err
		}
	}
	return nil, nil

}

func cleanHoundifyResponse(response string) string {
	// This should remove the "Redirected from" text
	re := regexp.MustCompile(`^Redirected from [^.]+\.\s*`)
	cleaned := re.ReplaceAllString(response, "")
	return cleaned
}

func houndifyTextRequest(queryText string, device string, session string) string {
	knowledge := vars.GetAPIConfig().Knowledge
	if !knowledge.Enable || knowledge.Provider != "houndify" {
		return "Houndify is not enabled."
	}

	logger.Println("Sending text request to Houndify...")

	req := houndifysdk.TextRequest{
		Query:     queryText,
		UserID:    device,
		RequestID: session,
	}

	serverResponse, err := HKGclient.TextSearch(req)
	if err != nil {
		logger.Println("Error sending text request to Houndify:", err)
		return ""
	}

	apiResponse, err := houndify.ParseSpokenResponse(serverResponse)
	if err != nil {
		logger.Println("Error parsing Houndify response:", err)
		logger.Println("Raw response:", serverResponse)
		return ""
	}

	apiResponse = cleanHoundifyResponse(apiResponse)

	logger.Println("Houndify response:", apiResponse)
	return apiResponse
}
