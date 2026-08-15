package wirepod_whisper

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
	"github.com/orcaman/writerseeker"
)

var Name string = "whisper"

// defaultBaseURL/defaultModel apply when the dashboard/env vars haven't set
// a value, matching the real OpenAI API. Any OpenAI-compatible server (e.g.
// a self-hosted faster-whisper-server) works by pointing BaseURL elsewhere.
const (
	defaultBaseURL = "https://api.openai.com"
	defaultModel   = "whisper-1"
)

type openAiResp struct {
	Text string `json:"text"`
}

// resolvedBaseURL/resolvedAPIKey/resolvedModel read live from
// vars.APIConfig so a dashboard change takes effect on the next request,
// without a restart.
func resolvedBaseURL() string {
	if v := vars.GetAPIConfig().STT.Whisper.BaseURL; v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultBaseURL
}

func resolvedAPIKey() string {
	if v := vars.GetAPIConfig().STT.Whisper.APIKey; v != "" {
		return v
	}
	// OPENAI_KEY is kept as a fallback for existing env-var-only setups.
	return os.Getenv("OPENAI_KEY")
}

func resolvedModel() string {
	if v := vars.GetAPIConfig().STT.Whisper.Model; v != "" {
		return v
	}
	return defaultModel
}

func Init() error {
	if resolvedAPIKey() == "" && resolvedBaseURL() == defaultBaseURL {
		logger.Println("Whisper STT: no API key configured and no custom endpoint set. Set one via the dashboard's STT Service settings, or the OPENAI_KEY/STT_WHISPER_KEY/STT_WHISPER_URL env vars, before selecting this backend.")
	}
	return nil
}

func pcm2wav(in io.Reader) []byte {

	// Output file.
	out := &writerseeker.WriterSeeker{}

	// 8 kHz, 16 bit, 1 channel, WAV.
	e := wav.NewEncoder(out, 16000, 16, 1, 1)

	// Create new audio.IntBuffer.
	audioBuf, err := newAudioIntBuffer(in)
	if err != nil {
		logger.Println(err)
	}
	// Write buffer to output file. This writes a RIFF header and the PCM chunks from the audio.IntBuffer.
	if err := e.Write(audioBuf); err != nil {
		logger.Println(err)
	}
	if err := e.Close(); err != nil {
		logger.Println(err)
	}
	outBuf := new(bytes.Buffer)
	io.Copy(outBuf, out.BytesReader())
	return outBuf.Bytes()
}

func newAudioIntBuffer(r io.Reader) (*audio.IntBuffer, error) {
	buf := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  16000,
		},
	}
	for {
		var sample int16
		err := binary.Read(r, binary.LittleEndian, &sample)
		switch {
		case err == io.EOF:
			return &buf, nil
		case err != nil:
			return nil, err
		}
		buf.Data = append(buf.Data, int(sample))
	}
}

func makeOpenAIReq(in []byte) string {
	url := resolvedBaseURL() + "/v1/audio/transcriptions"

	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)
	w.WriteField("model", resolvedModel())
	sendFile, _ := w.CreateFormFile("file", "audio.mp3")
	sendFile.Write(in)
	w.Close()

	httpReq, _ := http.NewRequest("POST", url, buf)
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	if key := resolvedAPIKey(); key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	// 60 second timeout for Whisper API (SEC-004)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		logger.Println(err)
		return "There was an error."
	}

	defer resp.Body.Close()

	response, _ := io.ReadAll(resp.Body)

	var aiResponse openAiResp
	json.Unmarshal(response, &aiResponse)

	return aiResponse.Text
}

func STT(req sr.SpeechRequest) (string, error) {
	logger.Println("(Bot " + req.Device + ", Whisper) Processing...")
	speechIsDone := false
	var err error
	for {
		_, err = req.GetNextStreamChunk()
		if err != nil {
			return "", err
		}
		if err != nil {
			return "", err
		}
		// has to be split into 320 []byte chunks for VAD
		speechIsDone, _ = req.DetectEndOfSpeech()
		if speechIsDone {
			break
		}
	}

	pcmBufTo := &writerseeker.WriterSeeker{}
	pcmBufTo.Write(req.DecodedMicData)
	pcmBuf := pcm2wav(pcmBufTo.BytesReader())

	transcribedText := strings.ToLower(makeOpenAIReq(pcmBuf))
	logger.Println("Bot " + req.Device + " Transcribed text: " + transcribedText)
	return transcribedText, nil
}
