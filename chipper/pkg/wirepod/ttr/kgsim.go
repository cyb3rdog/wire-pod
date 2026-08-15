package wirepod_ttr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/fforchino/vector-go-sdk/pkg/vector"
	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/sashabaranov/go-openai"
)

func GetChat(esn string) vars.RememberedChat {
	chats := vars.GetRememberedChats()
	for _, chat := range chats {
		if chat.ESN == esn {
			return chat
		}
	}
	return vars.RememberedChat{
		ESN: esn,
	}
}

func PlaceChat(chat vars.RememberedChat) {
	vars.UpdateRememberedChat(chat.ESN, chat.Chats)
}

// remember last 16 lines of chat
func Remember(user, ai openai.ChatCompletionMessage, esn string) {
	chatAppend := []openai.ChatCompletionMessage{
		user,
		ai,
	}
	currentChat := GetChat(esn)
	if len(currentChat.Chats) == 16 {
		var newChat vars.RememberedChat
		newChat.ESN = currentChat.ESN
		for i, chat := range currentChat.Chats {
			if i < 2 {
				continue
			}
			newChat.Chats = append(newChat.Chats, chat)
		}
		currentChat = newChat
	}
	currentChat.ESN = esn
	currentChat.Chats = append(currentChat.Chats, chatAppend...)
	PlaceChat(currentChat)
}

func isMn(r rune) bool {
	// Remove the characters that are not related to Vietnamese.
	// Retain the tonal marks and diacritics such as the circumflex, ơ, and ư in Vietnamese.
	keepMarks := []rune{
		'\u0300', // Dấu huyền
		'\u0301', // Dấu sắc
		'\u0303', // Dấu ngã
		'\u0309', // Dấu hỏi
		'\u0323', // Dấu nặng
		'\u0302', // Dấu mũ (â, ê, ô)
		'\u031B', // Dấu ơ và ư
		'\u0306', // Dấu trầm
	}
	if unicode.Is(unicode.Mn, r) {
		for _, mark := range keepMarks {
			if r == mark {
				return false
			}
		}
		return true
	}
	return false
}

func removeSpecialCharacters(str string) string {

	// these two lines create a transformation that decomposes characters, removes non-spacing marks (like diacritics), and then recomposes the characters, effectively removing special characters
	t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
	result, _, _ := transform.String(t, str)

	// Define the regular expression to match special characters
	re := regexp.MustCompile(`[&^*#@]`)

	// Replace special characters with an empty string
	result = removeEmojis(re.ReplaceAllString(result, ""))

	// Replace special characters with ASCII
	// * COPY/PASTE TO ADD MORE CHARACTERS:
	//   result = strings.ReplaceAll(result, "", "")
	result = strings.ReplaceAll(result, "‘", "'")
	result = strings.ReplaceAll(result, "’", "'")
	result = strings.ReplaceAll(result, "“", "\"")
	result = strings.ReplaceAll(result, "”", "\"")
	result = strings.ReplaceAll(result, "—", "-")
	result = strings.ReplaceAll(result, "–", "-")
	result = strings.ReplaceAll(result, "…", "...")
	result = strings.ReplaceAll(result, "\u00A0", " ")
	result = strings.ReplaceAll(result, "•", "*")
	result = strings.ReplaceAll(result, "¼", "1/4")
	result = strings.ReplaceAll(result, "½", "1/2")
	result = strings.ReplaceAll(result, "¾", "3/4")
	result = strings.ReplaceAll(result, "×", "x")
	result = strings.ReplaceAll(result, "÷", "/")
	result = strings.ReplaceAll(result, "ç", "c")
	result = strings.ReplaceAll(result, "©", "(c)")
	result = strings.ReplaceAll(result, "®", "(r)")
	result = strings.ReplaceAll(result, "™", "(tm)")
	result = strings.ReplaceAll(result, "@", "(a)")
	result = strings.ReplaceAll(result, " AI ", " A. I. ")
	return result
}

func removeEmojis(input string) string {
	// a mess, but it works!
	re := regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]|[\x{1F900}-\x{1F9FF}]|[\x{1F004}]|[\x{1F0CF}]|[\x{1F18E}]|[\x{1F191}-\x{1F251}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]|[\x{1F004}-\x{1F0CF}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]`)
	result := re.ReplaceAllString(input, "")
	return result
}

func CreateAIReq(transcribedText, esn string, gpt3tryagain, isKG bool) openai.ChatCompletionRequest {
	defaultPrompt := "You are a helpful, animated robot called Vector. Keep the response concise yet informative."

	var nChat []openai.ChatCompletionMessage

	smsg := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem,
	}
	if strings.TrimSpace(vars.APIConfig.Knowledge.OpenAIPrompt) != "" {
		smsg.Content = strings.TrimSpace(vars.APIConfig.Knowledge.OpenAIPrompt)
	} else {
		smsg.Content = defaultPrompt
	}

	var model string

	if gpt3tryagain {
		model = openai.GPT3Dot5Turbo
	} else if vars.APIConfig.Knowledge.Provider == "openai" {
		model = openai.GPT4oMini
		logger.Println("Using " + model)
	} else {
		logger.Println("Using " + vars.APIConfig.Knowledge.Model)
		model = vars.APIConfig.Knowledge.Model
	}

	smsg.Content = CreatePrompt(smsg.Content, model, isKG)

	nChat = append(nChat, smsg)
	if vars.APIConfig.Knowledge.SaveChat {
		rchat := GetChat(esn)
		logger.Println("Using remembered chats, length of " + fmt.Sprint(len(rchat.Chats)) + " messages")
		nChat = append(nChat, rchat.Chats...)
	}
	nChat = append(nChat, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: transcribedText,
	})

	aireq := openai.ChatCompletionRequest{
		Model:            model,
		MaxTokens:        2048,
		Temperature:      1,
		TopP:             1,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
		Messages:         nChat,
		Stream:           true,
	}
	return aireq
}

// logLLMError writes one detailed diagnostic line for a failed LLM call:
// which endpoint/model was actually hit, how long it took to fail, and --
// for a non-2xx HTTP response, the common case for a misconfigured or
// unreachable self-hosted/custom endpoint -- the real status code and
// response body go-openai captured, rather than just its default
// summarized error string (which for a RequestError already includes the
// body, but buried in one unlabeled line that's easy to miss, and for
// other error shapes doesn't surface it at all). Logged through both
// LogTray (docker logs / /api/get_debug_logs) and LogUI (dashboard
// "recent activity"), unlike the previous stdlib log.Printf call this
// replaces, which reached neither.
func logLLMError(what, endpoint, model string, elapsed time.Duration, err error) {
	var detail string
	var reqErr *openai.RequestError
	var apiErr *openai.APIError
	switch {
	case errors.As(err, &reqErr):
		body := strings.TrimSpace(string(reqErr.Body))
		if body == "" {
			body = "(empty)"
		} else if len(body) > 500 {
			body = body[:500] + "... (truncated)"
		}
		detail = fmt.Sprintf("HTTP %d (%s), response body: %s", reqErr.HTTPStatusCode, reqErr.HTTPStatus, body)
	case errors.As(err, &apiErr):
		detail = fmt.Sprintf("HTTP %d (%s): %s", apiErr.HTTPStatusCode, apiErr.HTTPStatus, apiErr.Message)
	default:
		detail = err.Error()
	}
	msg := fmt.Sprintf("LLM error (%s): endpoint=%s model=%s elapsed=%s -- %s",
		what, endpoint, model, elapsed.Round(time.Millisecond), detail)
	logger.Println(msg)
	logger.LogUI(msg)
}

func StreamingKGSim(req interface{}, esn string, transcribedText string, isKG bool) (string, error) {
	start := make(chan bool)
	stop := make(chan bool)
	stopStop := make(chan bool)
	kgReadyToAnswer := make(chan bool)
	kgStop := make(chan struct{})
	var kgStopOnce sync.Once
	stopKGAnim := func() { kgStopOnce.Do(func() { close(kgStop) }) }
	ctx := context.Background()
	matched := false
	var robot *vector.Vector
	var guid string
	var target string
	for _, bot := range vars.GetBotInfo().Robots {
		if esn == bot.Esn {
			guid = bot.GUID
			target = bot.IPAddress + ":443"
			matched = true
			break
		}
	}
	// robotAvailable gates every bit of this function that plays
	// animations or speaks the response through a direct connection back
	// to the robot's own local gRPC gateway (bot.IPAddress:443) -- a
	// separate connection from the one the robot itself opened to reach
	// wire-pod. That recorded IP is only trustworthy when wire-pod can
	// see the robot's real peer address; behind a reverse proxy (e.g.
	// Caddy TCP passthrough for a custom domain) it's the proxy's
	// address instead, and this dial ends up hitting wire-pod's own
	// front door by accident, which 502s it -- surfacing as a confusing
	// gRPC transport error with no connection to the LLM at all. Treat
	// every failure here as non-fatal: skip animation/speech feedback,
	// but still make the LLM call below, so a working LLM integration
	// stays verifiable independent of whether this side channel to the
	// robot works.
	robotAvailable := false
	if !vars.SDKEnabled() {
		logger.Println("(KG) SDK_ENABLED=false -- skipping robot control/animation feedback, LLM will still be called")
	} else if !matched {
		logger.Println("(KG) robot control unavailable (esn " + esn + " not registered) -- skipping animation feedback, LLM will still be called")
	} else {
		var err error
		robot, err = vector.New(vector.WithSerialNo(esn), vector.WithToken(guid), vector.WithTarget(target))
		if err != nil {
			logger.Println("(KG) robot control unavailable (" + err.Error() + ") -- skipping animation feedback, LLM will still be called")
		} else if _, err := robot.Conn.BatteryState(context.Background(), &vectorpb.BatteryStateRequest{}); err != nil {
			logger.Println("(KG) robot control unavailable (" + err.Error() + ") -- skipping animation feedback, LLM will still be called")
		} else {
			robotAvailable = true
		}
	}
	if isKG && robotAvailable {
		BControl(robot, ctx, start, stop)
		go func() {
			for {
				select {
				case <-kgStop:
					kgReadyToAnswer <- true
					return
				default:
				}
				robot.Conn.PlayAnimation(ctx, &vectorpb.PlayAnimationRequest{
					Animation: &vectorpb.Animation{
						Name: "anim_knowledgegraph_searching_01",
					},
					Loops: 1,
				})
				time.Sleep(time.Second / 3)
			}
		}()
	}
	var fullRespText string
	var fullfullRespText string
	var fullRespSlice []string
	var isDone bool
	var c *openai.Client
	llmEndpoint := "https://api.openai.com/v1"
	switch vars.APIConfig.Knowledge.Provider {
	case "together":
		if vars.APIConfig.Knowledge.Model == "" {
			vars.APIConfig.Knowledge.Model = "meta-llama/Llama-3-70b-chat-hf"
			if err := vars.WriteConfigToDisk(); err != nil {
				logger.Println("Failed to persist default Together model:", err)
			}
		}
		llmEndpoint = "https://api.together.xyz/v1"
		conf := openai.DefaultConfig(vars.APIConfig.Knowledge.Key)
		conf.BaseURL = llmEndpoint
		c = openai.NewClientWithConfig(conf)
	case "custom":
		llmEndpoint = vars.APIConfig.Knowledge.Endpoint
		conf := openai.DefaultConfig(vars.APIConfig.Knowledge.Key)
		conf.BaseURL = llmEndpoint
		c = openai.NewClientWithConfig(conf)
	case "openai":
		c = openai.NewClient(vars.APIConfig.Knowledge.Key)
	}
	speakReady := make(chan string)
	streamDone := make(chan struct{})
	var streamDoneOnce sync.Once
	closeStreamDone := func() { streamDoneOnce.Do(func() { close(streamDone) }) }
	successIntent := make(chan bool)

	aireq := CreateAIReq(transcribedText, esn, false, isKG)

	llmStart := time.Now()
	stream, err := c.CreateChatCompletionStream(ctx, aireq)
	if err != nil {
		logLLMError("creating chat completion stream", llmEndpoint, aireq.Model, time.Since(llmStart), err)
		if strings.Contains(err.Error(), "does not exist") && vars.APIConfig.Knowledge.Provider == "openai" {
			logger.Println("GPT-4 model cannot be accessed with this API key. You likely need to add more than $5 dollars of funds to your OpenAI account.")
			logger.LogUI("GPT-4 model cannot be accessed with this API key. You likely need to add more than $5 dollars of funds to your OpenAI account.")
			aireq := CreateAIReq(transcribedText, esn, true, isKG)
			logger.Println("Falling back to " + aireq.Model)
			logger.LogUI("Falling back to " + aireq.Model)
			llmStart = time.Now()
			stream, err = c.CreateChatCompletionStream(ctx, aireq)
			if err != nil {
				logLLMError("creating chat completion stream (fallback model)", llmEndpoint, aireq.Model, time.Since(llmStart), err)
				return "", err
			}
		} else {
			if isKG && robotAvailable {
				stopKGAnim()
				for range kgReadyToAnswer {
					break
				}
				stop <- true
				time.Sleep(time.Second / 3)
				KGSim(esn, "There was an error getting data from the L. L. M.")
			}
			return "", err
		}
	}
	nChat := aireq.Messages
	nChat = append(nChat, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
	})
	fmt.Println("LLM stream response: ")
	go func() {
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// prevents a crash
				if len(fullRespSlice) == 0 {
					trimmed := strings.TrimSpace(fullfullRespText)
					if trimmed == "" {
						msg := fmt.Sprintf("LLM returned no response: endpoint=%s model=%s elapsed=%s -- stream completed successfully but produced zero content chunks (check the model name, prompt/system-message compatibility, or whether the endpoint actually supports streaming chat completions)",
							llmEndpoint, aireq.Model, time.Since(llmStart).Round(time.Millisecond))
						logger.Println(msg)
						logger.LogUI(msg)
						successIntent <- false
						if isKG && robotAvailable {
							stopKGAnim()
							for range kgReadyToAnswer {
								break
							}
							stop <- true
							time.Sleep(time.Second / 3)
							KGSim(esn, "There was an error getting data from the L. L. M.")
						}
						break
					}
					// The LLM did respond, but the content never crossed a
					// sentence-ending punctuation mark (short replies with
					// no trailing '.'/'?'/'!' are common) -- the
					// sentence-splitting logic below never got a chance to
					// run, and this used to be discarded entirely as "no
					// response". Use the full response as the one and only
					// sentence instead of losing it.
					logger.Println("LLM debug: stream ended with no terminal punctuation, using full response as-is")
					fullRespSlice = append(fullRespSlice, trimmed)
					successIntent <- true
				}
				isDone = true
				// if fullRespSlice != fullRespText, add that missing bit to fullRespSlice
				newStr := fullRespSlice[0]
				for i, str := range fullRespSlice {
					if i == 0 {
						continue
					}
					newStr = newStr + " " + str
				}
				if strings.TrimSpace(newStr) != strings.TrimSpace(fullfullRespText) {
					logger.Println("LLM debug: there is content after the last punctuation mark")
					extraBit := strings.TrimPrefix(fullRespText, newStr)
					fullRespSlice = append(fullRespSlice, extraBit)
				}
				if vars.APIConfig.Knowledge.SaveChat {
					Remember(openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleUser,
						Content: transcribedText,
					},
						openai.ChatCompletionMessage{
							Role:    openai.ChatMessageRoleAssistant,
							Content: newStr,
						},
						esn)
				}
				logger.LogUI("LLM response for " + esn + ": " + newStr)
				logger.Println("LLM stream finished in " + time.Since(llmStart).Round(time.Millisecond).String())
				closeStreamDone()
				return
			}

			if err != nil {
				logLLMError("reading chat completion stream", llmEndpoint, aireq.Model, time.Since(llmStart), err)
				isDone = true // no more content will arrive
				select {
				case successIntent <- false:
				default:
				}
				closeStreamDone()
				return
			}

			if len(response.Choices) == 0 {
				// Together AI and some providers send empty-Choices delta chunks as
				// stream init/heartbeat artifacts — skip them, do not abort.
				continue
			}

			fullfullRespText = fullfullRespText + removeSpecialCharacters(response.Choices[0].Delta.Content)
			fullRespText = fullRespText + removeSpecialCharacters(response.Choices[0].Delta.Content)
			if strings.Contains(fullRespText, "...") || strings.Contains(fullRespText, ".'") || strings.Contains(fullRespText, ".\"") || strings.Contains(fullRespText, ".") || strings.Contains(fullRespText, "?") || strings.Contains(fullRespText, "!") {
				var sepStr string
				if strings.Contains(fullRespText, "...") {
					sepStr = "..."
				} else if strings.Contains(fullRespText, ".'") {
					sepStr = ".'"
				} else if strings.Contains(fullRespText, ".\"") {
					sepStr = ".\""
				} else if strings.Contains(fullRespText, ".") {
					sepStr = "."
				} else if strings.Contains(fullRespText, "?") {
					sepStr = "?"
				} else if strings.Contains(fullRespText, "!") {
					sepStr = "!"
				}
				splitResp := strings.Split(strings.TrimSpace(fullRespText), sepStr)
				fullRespSlice = append(fullRespSlice, strings.TrimSpace(splitResp[0])+sepStr)
				fullRespText = splitResp[1]
				select {
				case successIntent <- true:
				default:
				}
				select {
				case speakReady <- strings.TrimSpace(splitResp[0]) + sepStr:
				default:
				}
			}
		}
	}()
	for is := range successIntent {
		if is {
			if !isKG {
				IntentPass(req, "intent_greeting_hello", transcribedText, map[string]string{}, false)
			}
			break
		} else {
			return "", errors.New("llm returned no response")
		}
	}
	if !robotAvailable {
		// No connection to speak the response through -- but the LLM
		// call itself already succeeded (successIntent came back true)
		// and is still streaming. Wait for it to finish so the full
		// response is captured, and return it directly instead of
		// running the robot-driven playback loop below, none of which
		// is reachable without robot.Conn.
		<-streamDone
		logger.Println("(KG) robot control unavailable -- LLM responded in " + time.Since(llmStart).Round(time.Millisecond).String() + ": " + fullfullRespText)
		return fullfullRespText, nil
	}
	time.Sleep(time.Millisecond * 200)
	if !isKG {
		BControl(robot, ctx, start, stop)
	}
	interrupted := false
	interruptedCh := make(chan struct{})
	go func() {
		if InterruptKGSimWhenTouchedOrWaked(robot, stop, stopStop) {
			interrupted = true
			close(interruptedCh)
		}
	}()
	var TTSLoopAnimation string
	var TTSGetinAnimation string
	if isKG {
		TTSLoopAnimation = "anim_knowledgegraph_answer_01"
		TTSGetinAnimation = "anim_knowledgegraph_searching_getout_01"
	} else {
		TTSLoopAnimation = "anim_tts_loop_02"
		TTSGetinAnimation = "anim_getin_tts_01"
	}

	stopTTSLoopCh := make(chan struct{})
	TTSLoopStopped := make(chan bool)
	for range start {
		if isKG {
			stopKGAnim()
			for range kgReadyToAnswer {
				break
			}
		} else {
			time.Sleep(time.Millisecond * 300)
		}
		robot.Conn.PlayAnimation(
			ctx,
			&vectorpb.PlayAnimationRequest{
				Animation: &vectorpb.Animation{
					Name: TTSGetinAnimation,
				},
				Loops: 1,
			},
		)
		if !vars.APIConfig.Knowledge.CommandsEnable {
			go func() {
				for {
					select {
					case <-stopTTSLoopCh:
						TTSLoopStopped <- true
						return
					default:
					}
					robot.Conn.PlayAnimation(
						ctx,
						&vectorpb.PlayAnimationRequest{
							Animation: &vectorpb.Animation{
								Name: TTSLoopAnimation,
							},
							Loops: 1,
						},
					)
				}
			}()
		}
		var disconnect bool
		numInResp := 0
	outerLoop:
		for {
			respSlice := fullRespSlice
			if len(respSlice)-1 < numInResp {
				if !isDone {
					logger.Println("Waiting for more content from LLM...")
					select {
					case <-speakReady:
						// new sentence added; restart loop to re-snapshot fullRespSlice
					case <-streamDone:
						// stream finished (isDone=true or error); restart loop to hit break
					case <-interruptedCh:
						break outerLoop
					}
					continue // re-snapshot respSlice at top and re-check condition
				} else {
					break outerLoop
				}
			}
			if interrupted {
				break outerLoop
			}
			logger.Println(respSlice[numInResp])
			acts := GetActionsFromString(respSlice[numInResp])
			nChat[len(nChat)-1].Content = fullRespText
			disconnect = PerformActions(nChat, acts, robot, stopStop)
			if disconnect {
				break outerLoop
			}
			numInResp = numInResp + 1
		}
		if !vars.APIConfig.Knowledge.CommandsEnable {
			close(stopTTSLoopCh)
			for range TTSLoopStopped {
				break
			}
		}
		time.Sleep(time.Millisecond * 100)
		// if isKG {
		// 	robot.Conn.PlayAnimation(
		// 		ctx,
		// 		&vectorpb.PlayAnimationRequest{
		// 			Animation: &vectorpb.Animation{
		// 				Name: "anim_knowledgegraph_success_01",
		// 			},
		// 			Loops: 1,
		// 		},
		// 	)
		// 	time.Sleep(time.Millisecond * 3300)
		// }
		close(stopStop)
		if !interrupted {
			stop <- true
		}
	}
	return "", nil
}

func KGSim(esn string, textToSay string) error {
	ctx := context.Background()
	matched := false
	var robot *vector.Vector
	var guid string
	var target string
	for _, bot := range vars.GetBotInfo().Robots {
		if esn == bot.Esn {
			guid = bot.GUID
			target = bot.IPAddress + ":443"
			matched = true
			break
		}
	}
	if matched {
		var err error
		robot, err = vector.New(vector.WithSerialNo(esn), vector.WithToken(guid), vector.WithTarget(target))
		if err != nil {
			return err
		}
	}
	controlRequest := &vectorpb.BehaviorControlRequest{
		RequestType: &vectorpb.BehaviorControlRequest_ControlRequest{
			ControlRequest: &vectorpb.ControlRequest{
				Priority: vectorpb.ControlRequest_OVERRIDE_BEHAVIORS,
			},
		},
	}
	go func() {
		start := make(chan bool)
		stop := make(chan bool)

		go func() {
			// * begin - modified from official vector-go-sdk
			r, err := robot.Conn.BehaviorControl(
				ctx,
			)
			if err != nil {
				log.Println(err)
				return
			}

			if err := r.Send(controlRequest); err != nil {
				log.Println(err)
				return
			}

			for {
				ctrlresp, err := r.Recv()
				if err != nil {
					log.Println(err)
					return
				}
				if ctrlresp.GetControlGrantedResponse() != nil {
					start <- true
					break
				}
			}

			for {
				select {
				case <-stop:
					logger.Println("KGSim: releasing behavior control (interrupt)")
					if err := r.Send(
						&vectorpb.BehaviorControlRequest{
							RequestType: &vectorpb.BehaviorControlRequest_ControlRelease{
								ControlRelease: &vectorpb.ControlRelease{},
							},
						},
					); err != nil {
						log.Println(err)
						return
					}
					return
				default:
					continue
				}
			}
			// * end - modified from official vector-go-sdk
		}()

		stopTTSLoopCh := make(chan struct{})
		TTSLoopStopped := make(chan bool)
		for range start {
			time.Sleep(time.Millisecond * 300)
			robot.Conn.PlayAnimation(
				ctx,
				&vectorpb.PlayAnimationRequest{
					Animation: &vectorpb.Animation{
						Name: "anim_getin_tts_01",
					},
					Loops: 1,
				},
			)
			go func() {
				for {
					select {
					case <-stopTTSLoopCh:
						TTSLoopStopped <- true
						return
					default:
					}
					robot.Conn.PlayAnimation(
						ctx,
						&vectorpb.PlayAnimationRequest{
							Animation: &vectorpb.Animation{
								Name: "anim_tts_loop_02",
							},
							Loops: 1,
						},
					)
				}
			}()
			var stopSent bool
			textToSaySplit := strings.Split(textToSay, ". ")
			for _, str := range textToSaySplit {
				_, err := robot.Conn.SayText(
					ctx,
					&vectorpb.SayTextRequest{
						Text:           str + ".",
						UseVectorVoice: true,
						DurationScalar: 1.0,
					},
				)
				if err != nil {
					logger.Println("KG SayText error: " + err.Error())
					// Release behavior control immediately on error; guard against
					// the second stop send below so we don't block forever with no receiver.
					stop <- true
					stopSent = true
					break
				}
			}
			close(stopTTSLoopCh)
			for range TTSLoopStopped {
				break
			}
			time.Sleep(time.Millisecond * 100)
			if !stopSent {
				stop <- true
			}
		}
	}()
	return nil
}
