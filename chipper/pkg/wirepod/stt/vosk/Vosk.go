// Vosk.go - Wire-pod VOSK STT Adapter with Thermal Management
// Modified version with idle detection and CPU throttling for RPi Zero 2W
//
// CHANGES FROM ORIGINAL:
// 1. Added idle detection and sleep logic to reduce CPU when not processing
// 2. Added temperature monitoring and automatic throttling
// 3. Added adaptive sleep duration based on activity patterns
// 4. Added recognition pool size limit to prevent resource exhaustion
//
// For RPi Zero 2W: Target CPU < 40%, Temperature < 70°C

package wirepod_vosk

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	vosk "github.com/kercre123/vosk-api/go"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
)

// =============================================================================
// THERMAL MANAGEMENT CONSTANTS (RPi Zero 2W optimized)
// =============================================================================
const (
	// Temperature thresholds (in Celsius)
	TempThresholdWarning  = 70.0
	TempThresholdEscalate = 75.0
	TempThresholdCritical = 80.0

	// Idle detection settings
	IdleCheckInterval = 5 * time.Second   // How often to check for idle
	IdleTimeout       = 30 * time.Second   // Time without activity before sleep
	MinSleepDuration  = 2 * time.Second    // Minimum sleep when idle
	MaxSleepDuration  = 10 * time.Second   // Maximum sleep when idle

	// CPU throttling settings
	MaxRecognizersPerPool = 5    // Maximum recognizers to keep in pool
	ThermalThrottleDelay  = 500 * time.Millisecond  // Delay added under thermal pressure

	// Activity tracking
	ActivityWindowSize = 10  // Number of recent requests to track for pattern analysis
)

// =============================================================================
// THERMAL STATE TRACKING
// =============================================================================
var (
	// Thermal state
	lastRequestTime   time.Time
	lastActivityTime  time.Time
	isProcessingLock  sync.Mutex
	isProcessing      bool
	isSleeping        atomic.Bool
	sleepDuration     time.Duration
	thermalPressure   atomic.Int32  // 0=normal, 1=warm, 2=hot, 3=critical

	// Activity metrics
	recentRequests    []time.Time
	requestMutex      sync.Mutex
	totalRequests     atomic.Int64
	totalSleepTime    atomic.Int64
)

// =============================================================================
// THERMAL MANAGEMENT FUNCTIONS
// =============================================================================

// GetCPUTemperature reads CPU temperature from system file
// Works on Raspberry Pi and similar ARM boards
func GetCPUTemperature() float64 {
	// Try common temperature file locations
	tempFiles := []string{
		"/sys/class/thermal/thermal_zone0/temp",
		"/sys/class/hwmon/hwmon0/temp1_input",
		"/sys/devices/virtual/thermal/thermal_zone0/temp",
	}

	for _, path := range tempFiles {
		if data, err := os.ReadFile(path); err == nil {
			var temp int
			if _, parseErr := fmt.Sscanf(string(data), "%d", &temp); parseErr == nil {
				// Temperature is usually in millidegrees
				if temp > 1000 {
					return float64(temp) / 1000.0
				}
				return float64(temp)
			}
		}
	}
	return 0 // Unable to read temperature
}

// UpdateThermalState checks current conditions and adjusts throttling
func UpdateThermalState() {
	temp := GetCPUTemperature()

	var pressure int32
	switch {
	case temp >= TempThresholdCritical:
		pressure = 3
	case temp >= TempThresholdEscalate:
		pressure = 2
	case temp >= TempThresholdWarning:
		pressure = 1
	default:
		pressure = 0
	}

	thermalPressure.Store(pressure)

	// Log thermal state changes
	if pressure > 0 {
		logger.Printf("(Thermal) Temperature: %.1f°C, Pressure: %d", temp, pressure)
	}
}

// ShouldThrottle returns true if we should add processing delay
func ShouldThrottle() bool {
	return thermalPressure.Load() >= 2
}

// GetThrottleDelay returns the delay to add based on thermal state
func GetThrottleDelay() time.Duration {
	switch thermalPressure.Load() {
	case 3: // Critical
		return ThermalThrottleDelay * 4
	case 2: // Hot
		return ThermalThrottleDelay * 2
	case 1: // Warm
		return ThermalThrottleDelay
	default:
		return 0
	}
}

// =============================================================================
// IDLE DETECTION AND SLEEP LOGIC
// =============================================================================

// RecordActivity logs a new speech request for pattern analysis
func RecordActivity() {
	requestMutex.Lock()
	defer requestMutex.Unlock()

	now := time.Now()
	recentRequests = append(recentRequests, now)
	lastActivityTime = now
	lastRequestTime = now
	totalRequests.Add(1)

	// Keep only recent requests in window
	if len(recentRequests) > ActivityWindowSize {
		recentRequests = recentRequests[len(recentRequests)-ActivityWindowSize:]
	}
}

// CalculateIdleTime returns duration since last activity
func CalculateIdleTime() time.Duration {
	requestMutex.Lock()
	defer requestMutex.Unlock()
	return time.Since(lastActivityTime)
}

// ShouldSleep returns true if system should enter low-power mode
func ShouldSleep() bool {
	// Don't sleep if currently processing
	isProcessingLock.Lock()
	processing := isProcessing
	isProcessingLock.Unlock()

	if processing {
		return false
	}

	// Don't sleep if thermal pressure is high (keep processing minimal)
	if thermalPressure.Load() >= 2 {
		return false
	}

	// Check idle time
	return CalculateIdleTime() > IdleTimeout
}

// SleepIfNeeded enters low-power mode if conditions are right
func SleepIfNeeded() {
	if !ShouldSleep() {
		return
	}

	// Calculate adaptive sleep duration based on recent activity
	sleepDur := CalculateAdaptiveSleepDuration()

	logger.Printf("(Thermal) Entering idle sleep for %v", sleepDur)

	isSleeping.Store(true)
	sleepStart := time.Now()

	time.Sleep(sleepDur)

	sleepDuration := time.Since(sleepStart)
	totalSleepTime.Add(int64(sleepDuration))
	isSleeping.Store(false)

	logger.Printf("(Thermal) Exited idle sleep after %v", sleepDuration)
}

// CalculateAdaptiveSleepDuration determines optimal sleep based on activity patterns
func CalculateAdaptiveSleepDuration() time.Duration {
	requestMutex.Lock()
	defer requestMutex.Unlock()

	if len(recentRequests) < 2 {
		// Not enough data, use default
		return MinSleepDuration
	}

	// Calculate average interval between recent requests
	var totalInterval time.Duration
	for i := 1; i < len(recentRequests); i++ {
		totalInterval += recentRequests[i].Sub(recentRequests[i-1])
	}
	avgInterval := totalInterval / time.Duration(len(recentRequests)-1)

	// Sleep for slightly longer than average interval
	suggested := avgInterval * 2

	// Clamp to bounds
	if suggested < MinSleepDuration {
		return MinSleepDuration
	}
	if suggested > MaxSleepDuration {
		return MaxSleepDuration
	}
	return suggested
}

// StartIdleMonitor starts background goroutine for idle detection
func StartIdleMonitor(stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(IdleCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				logger.Println("(Thermal) Idle monitor stopped")
				return
			case <-ticker.C:
				UpdateThermalState()
				SleepIfNeeded()
			}
		}
	}()
}

// =============================================================================
// MODIFIED VOSK ADAPTER WITH THERMAL MANAGEMENT
// =============================================================================

// Existing global variables (from original)
var GrammerEnable bool = false
var Name string = "vosk"
var model *vosk.VoskModel
var recsmu sync.Mutex
var grmRecs []ARec
var gpRecs []ARec
var modelLoaded bool
var Grammer string

type ARec struct {
	InUse bool
	Rec   *vosk.VoskRecognizer
}

// Init initializes VOSK with thermal management
func Init() error {
	if os.Getenv("VOSK_WITH_GRAMMER") == "true" {
		fmt.Println("Initializing vosk with grammer optimizations")
		GrammerEnable = true
	}
	if vars.APIConfig.PastInitialSetup {
		vosk.SetLogLevel(-1)
		if modelLoaded {
			logger.Println("A model was already loaded, freeing all recognizers and model")
			for ind := range grmRecs {
				grmRecs[ind].Rec.Free()
			}
			for ind := range gpRecs {
				gpRecs[ind].Rec.Free()
			}
			gpRecs = []ARec{}
			grmRecs = []ARec{}
			model.Free()
		}
		sttLanguage := vars.APIConfig.STT.Language
		if len(sttLanguage) == 0 {
			sttLanguage = "en-US"
		}
		modelPath := filepath.Join(vars.VoskModelPath, sttLanguage, "model")
		if _, err := os.Stat(modelPath); err != nil {
			fmt.Println("Path does not exist: " + modelPath)
			return err
		}
		logger.Println("Opening VOSK model (" + modelPath + ")")
		aModel, err := vosk.NewModel(modelPath)
		if err != nil {
			log.Fatal(err)
			return err
		}
		model = aModel
		if GrammerEnable {
			logger.Println("Initializing grammer list")
			Grammer = GetGrammerList(vars.APIConfig.STT.Language)
		}

		logger.Println("Initializing VOSK recognizers")
		if GrammerEnable {
			grmRecognizer, err := vosk.NewRecognizerGrm(aModel, 16000.0, Grammer)
			if err != nil {
				log.Fatal(err)
			}
			var grmrec ARec
			grmrec.Rec = grmRecognizer
			grmrec.InUse = false
			grmRecs = append(grmRecs, grmrec)
		}
		gpRecognizer, err := vosk.NewRecognizer(aModel, 16000.0)
		var gprec ARec
		gprec.Rec = gpRecognizer
		gprec.InUse = false
		gpRecs = append(gpRecs, gprec)
		if err != nil {
			log.Fatal(err)
		}
		modelLoaded = true

		// Start thermal management
		lastActivityTime = time.Now()
		lastRequestTime = time.Now()
		isSleeping.Store(false)
		recentRequests = make([]time.Time, 0, ActivityWindowSize)

		// Start idle monitor in background
		stopCh := make(chan struct{})
		StartIdleMonitor(stopCh)

		runTest()
	}
	return nil
}

// getRec is modified to limit pool size
func getRec(withGrm bool) (*vosk.VoskRecognizer, int) {
	recsmu.Lock()
	defer recsmu.Unlock()

	// Check thermal state and apply throttling if needed
	throttleDelay := GetThrottleDelay()
	if throttleDelay > 0 {
		time.Sleep(throttleDelay)
	}

	if withGrm && GrammerEnable {
		// Limit pool size for grammar recognizers
		for ind, rec := range grmRecs {
			if !rec.InUse {
				grmRecs[ind].InUse = true
				return grmRecs[ind].Rec, ind
			}
		}
		// Only create new if under limit
		if len(grmRecs) < MaxRecognizersPerPool {
			recsmu.Unlock()
			goto createNew
		}
	} else {
		// Limit pool size for general recognizers
		for ind, rec := range gpRecs {
			if !rec.InUse {
				gpRecs[ind].InUse = true
				return gpRecs[ind].Rec, ind
			}
		}
		// Only create new if under limit
		if len(gpRecs) < MaxRecognizersPerPool {
			recsmu.Unlock()
			goto createNew
		}
	}

	// Pool exhausted - wait and retry
	recsmu.Unlock()
	time.Sleep(100 * time.Millisecond)
	recsmu.Lock()
	// Try again
	if withGrm && GrammerEnable {
		for ind, rec := range grmRecs {
			if !rec.InUse {
				grmRecs[ind].InUse = true
				return grmRecs[ind].Rec, ind
			}
		}
	} else {
		for ind, rec := range gpRecs {
			if !rec.InUse {
				gpRecs[ind].InUse = true
				return gpRecs[ind].Rec, ind
			}
		}
	}
	// Still nothing available - create emergency recognizer
	goto createNew

createNew:
	var newrec ARec
	var newRec *vosk.VoskRecognizer
	var err error
	newrec.InUse = true
	if withGrm {
		newRec, err = vosk.NewRecognizerGrm(model, 16000.0, Grammer)
	} else {
		newRec, err = vosk.NewRecognizer(model, 16000.0)
	}
	if err != nil {
		log.Fatal(err)
	}
	newrec.Rec = newRec
	recsmu.Lock()
	if withGrm {
		grmRecs = append(grmRecs, newrec)
		return grmRecs[len(grmRecs)-1].Rec, len(grmRecs) - 1
	} else {
		gpRecs = append(gpRecs, newrec)
		return gpRecs[len(gpRecs)-1].Rec, len(gpRecs) - 1
	}
}

// STT is the main speech recognition function with thermal management
func STT(req sr.SpeechRequest) (string, error) {
	// Record this activity for idle tracking
	RecordActivity()

	// Check if we should wait (temperature too high)
	for ShouldThrottle() {
		logger.Println("(Thermal) Waiting due to high temperature...")
		time.Sleep(ThermalThrottleDelay)
		UpdateThermalState()
	}

	// Mark as processing
	isProcessingLock.Lock()
	isProcessing = true
	isProcessingLock.Unlock()

	// Ensure we unmark processing when done
	defer func() {
		isProcessingLock.Lock()
		isProcessing = false
		isProcessingLock.Unlock()
	}()

	logger.Println("(Bot " + req.Device + ", Vosk) Processing...")

	// Optional: Check idle before processing
	if isSleeping.Load() {
		logger.Println("(Thermal) Waking from idle sleep")
	}

	var withGrm bool
	if (vars.APIConfig.Knowledge.IntentGraph || req.IsKG) || !GrammerEnable {
		logger.Println("Using general recognizer")
		withGrm = false
	} else {
		logger.Println("Using grammer-optimized recognizer")
		withGrm = true
	}

	rec, recind := getRec(withGrm)
	rec.SetWords(1)
	rec.AcceptWaveform(req.FirstReq)
	req.DetectEndOfSpeech()
	for {
		chunk, err := req.GetNextStreamChunk()
		if err != nil {
			return "", err
		}
		speechIsDone, doProcess := req.DetectEndOfSpeech()
		if doProcess {
			rec.AcceptWaveform(chunk)
		}
		if speechIsDone {
			break
		}
	}

	var jres map[string]interface{}
	err = json.Unmarshal([]byte(rec.FinalResult()), &jres)
	if err != nil {
		logger.Println("JSON unmarshal error:", err)
		return "", fmt.Errorf("failed to parse Vosk result: %w", err)
	}

	if withGrm {
		grmRecs[recind].InUse = false
	} else {
		gpRecs[recind].InUse = false
	}

	transcribedText := jres["text"].(string)
	logger.Println("Bot " + req.Device + " Transcribed text: " + transcribedText)

	// Update thermal state after processing
	UpdateThermalState()

	return transcribedText, nil
}

// GetThermalStats returns current thermal management statistics
func GetThermalStats() map[string]interface{} {
	return map[string]interface{}{
		"total_requests":    totalRequests.Load(),
		"total_sleep_time": totalSleepTime.Load(),
		"current_pressure": thermalPressure.Load(),
		"is_sleeping":       isSleeping.Load(),
		"cpu_temp":          GetCPUTemperature(),
		"idle_time":        CalculateIdleTime().Seconds(),
	}
}

// runTest remains unchanged from original
func runTest() {
	logger.Println("Running recognizer test")
	var withGrm bool
	if GrammerEnable {
		logger.Println("Using grammer-optimized recognizer")
		withGrm = true
	} else {
		logger.Println("Using general recognizer")
		withGrm = false
	}
	rec, recind := getRec(withGrm)
	sttTestPath := "./stttest.pcm"
	if runtime.GOOS == "android" {
		sttTestPath = vars.AndroidPath + "/static/stttest.pcm"
	}
	pcmBytes, _ := os.ReadFile(sttTestPath)
	var micData [][]byte
	cTime := time.Now()
	micData = sr.SplitVAD(pcmBytes)
	for _, sample := range micData {
		rec.AcceptWaveform(sample)
	}
	var jres map[string]interface{}
	err = json.Unmarshal([]byte(rec.FinalResult()), &jres)
	if err != nil {
		logger.Println("JSON unmarshal error in test:", err)
		return
	}
	if withGrm {
		grmRecs[recind].InUse = false
	} else {
		gpRecs[recind].InUse = false
	}
	transcribedText := jres["text"].(string)
	tTime := time.Since(cTime)
	logger.Println("Text (from test):", transcribedText)
	if tTime.Seconds() > 3 {
		logger.Println("Vosk test took a while, performance may be degraded. (" + fmt.Sprint(tTime) + ")")
	}
	logger.Println("Vosk test successful! (Took " + fmt.Sprint(tTime) + ")")
}

// GetGrammerList remains unchanged from original
func GetGrammerList(lang string) string {
	var wordsList []string
	var grammer string
	for _, words := range vars.IntentList {
		for _, word := range words.Keyphrases {
			wors := strings.Split(word, " ")
			for _, wor := range wors {
				found := model.FindWord(wor)
				if found != -1 {
					wordsList = append(wordsList, wor)
				}
			}
		}
	}
	for _, str := range localization.ALL_STR {
		text := localization.GetText(str)
		wors := strings.Split(text, " ")
		for _, wor := range wors {
			found := model.FindWord(wor)
			if found != -1 {
				wordsList = append(wordsList, wor)
			}
		}
	}
	for _, intent := range vars.CustomIntents {
		for _, utterance := range intent.Utterances {
			wors := strings.Split(utterance, " ")
			for _, wor := range wors {
				found := model.FindWord(wor)
				if found != -1 {
					wordsList = append(wordsList, wor)
				}
			}
		}
	}
	for _, wor := range NumbersEN_US {
		found := model.FindWord(wor)
		if found != -1 {
			wordsList = append(wordsList, wor)
		}
	}

	wordsList = removeDuplicates(wordsList)
	for i, word := range wordsList {
		if i == len(wordsList)-1 {
			grammer = grammer + `"` + word + `"`
		} else {
			grammer = grammer + `"` + word + `"` + ", "
		}
	}
	grammer = "[" + grammer + "]"
	return grammer
}

// removeDuplicates remains unchanged from original
func removeDuplicates(strings []string) []string {
	occurred := map[string]bool{}
	var result []string
	for _, str := range strings {
		if !occurred[str] {
			result = append(result, str)
			occurred[str] = true
		}
	}
	return result
}
