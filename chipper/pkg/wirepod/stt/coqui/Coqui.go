package wirepod_coqui

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/asticode/go-asticoqui"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
)

var Name string = "coqui"

// coquiModel is loaded once in Init() and reused for every request.
// Previously STT() reloaded the model and scorer from disk on every single
// call, adding tens to hundreds of milliseconds of avoidable latency to
// every utterance -- the same class of load Vosk's recognizer pool already
// pays once, not per request.
var coquiModel *asticoqui.Model

// newStreamMu serializes stream creation against the shared model. The
// underlying C++ library's per-request state (a Stream) is documented as
// safe to run concurrently once created; this only guards the brief
// moment of creating one, not the actual audio feed/decode loop.
var newStreamMu sync.Mutex

func loadModel() (*asticoqui.Model, error) {
	model, err := asticoqui.New("../stt/model.tflite")
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat("../stt/large_vocabulary.scorer"); err == nil {
		model.EnableExternalScorer("../stt/large_vocabulary.scorer")
	} else if _, err := os.Stat("../stt/model.scorer"); err == nil {
		model.EnableExternalScorer("../stt/model.scorer")
	} else {
		logger.Println("No .scorer file found.")
	}
	return model, nil
}

func newStream() (*asticoqui.Stream, error) {
	newStreamMu.Lock()
	defer newStreamMu.Unlock()
	return coquiModel.NewStream()
}

// Init should be defined as `func() error`
func Init() error {
	logger.Println("Running a Coqui test...")
	model, err := loadModel()
	if err != nil {
		log.Fatal(err)
	}
	coquiModel = model

	coquiStream, err := newStream()
	if err != nil {
		log.Fatal(err)
	}
	pcmBytes, _ := os.ReadFile("./stttest.pcm")
	var micData [][]byte
	cTime := time.Now()
	micData = sr.SplitVAD(pcmBytes)
	for _, sample := range micData {
		coquiStream.FeedAudioContent(sr.BytesToSamples(sample))
	}
	res, err := coquiStream.Finish()
	tTime := time.Since(cTime)
	if err != nil {
		log.Fatal("Failed testing speech to text: ", err)
	}
	logger.Println("Text:", res)
	if tTime.Seconds() > 3 {
		logger.Println("Coqui test took a while, performance may be degraded. (" + fmt.Sprint(tTime) + ")")
	}
	logger.Println("Coqui test successful! (Took " + fmt.Sprint(tTime) + ")")
	return nil
}

// STT funcs should be defined as func(sr.SpeechRequest) (string, error)

func STT(req sr.SpeechRequest) (string, error) {
	logger.Println("(Bot " + req.Device + ", Coqui) Processing...")
	speechIsDone := false
	coquiStream, err := newStream()
	if err != nil {
		return "", err
	}
	for {
		var chunk []byte
		chunk, err = req.GetNextStreamChunk()
		if err != nil {
			return "", err
		}
		coquiStream.FeedAudioContent(sr.BytesToSamples(chunk))
		speechIsDone, _ = req.DetectEndOfSpeech()
		if speechIsDone {
			break
		}
	}
	transcribedText, err := coquiStream.Finish()
	if err != nil {
		return "", err
	}
	logger.Println("Bot " + req.Device + " Transcribed text: " + transcribedText)
	return transcribedText, nil
}
