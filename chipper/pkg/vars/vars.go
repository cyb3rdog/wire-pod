package vars

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/fforchino/vector-go-sdk/pkg/vector"
	"github.com/kercre123/wire-pod/chipper/pkg/fileutil"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/sashabaranov/go-openai"
	"github.com/wlynxg/anet"
)

var CommitSHA string

// initialize variables so they don't have to be found during runtime

var VarsInited bool

// if compiled into an installation package. wire-pod will use os.UserConfigDir()
var Packaged bool

var IsPackagedLinux bool

var AndroidPath string

var (
	JdocsPath         string = "./jdocs/jdocs.json"
	JdocsDir          string = "./jdocs"
	CustomIntentsPath string = "./customIntents.json"
	BotInfoPath       string = "./jdocs/botSdkInfo.json"
	BotInfoName       string = "botSdkInfo.json"
	PodName           string = "wire-pod"
	VoskModelPath     string = "../vosk/models/"
	WhisperModelPath  string = "../whisper.cpp/models/"
	SessionCertPath   string = "./session-certs/"
	VersionFile       string = "./version"
)

var (
	OutboundIPTester = "8.8.8.8:80"
	CertPath         = "../certs/cert.crt"
	KeyPath          = "../certs/cert.key"
	ServerConfigPath = "../certs/server_config.json"
	Certs            = "../certs"
)

var WebPort string = "8080"

// SDKEnabled reports whether wire-pod's SDK app server (port 80: bot
// remote-control features -- eye color, volume, Alexa opt-in, camera
// streaming -- plus the jdocs pinger and its own connCheck-triggered mDNS
// discovery) should start at all. It's independent of the core voice
// pipeline (the chipper/jdocs/token gRPC services on :443, STT, intent
// matching, and the knowledge-graph fallback never import this package),
// so disabling it -- e.g. to run wire-pod as a voice-only cloud server
// behind a reverse proxy exposing just :443 -- doesn't affect any of
// that. No dashboard setting backs this; like DISABLE_MDNS/
// VOSK_THERMAL_MANAGEMENT it's read live, so it applies on every restart.
func SDKEnabled() bool {
	return os.Getenv("SDK_ENABLED") != "false"
}

// /home/name/.anki_vector/
var SDKIniPath string
var BotJdocs []botjdoc
var BotInfo RobotInfoStore
var CustomIntents []CustomIntent
var CustomIntentsExist bool = false
var DownloadedVoskModels []string
var VoskGrammerEnable bool = false

// Mutexes for thread-safe access to shared slices
var botJdocsMu sync.RWMutex // Protects BotJdocs
var rememberedChatsMu sync.RWMutex
var recurringInfoMu sync.RWMutex // Protects RecurringInfo
var botInfoMu sync.RWMutex       // Protects BotInfo

// here to prevent import cycle (localization restructure)
var SttInitFunc func() error

var IntentList []JsonIntent

//var MatchListList [][]string
// var IntentsList = []string{}

var ChipperCert []byte
var ChipperKey []byte
var ChipperKeysLoaded bool

var RecurringInfo []RecurringInfoStore

// Thread-safe accessors for RememberedChats
func GetRememberedChats() []RememberedChat {
	rememberedChatsMu.RLock()
	defer rememberedChatsMu.RUnlock()
	// Return a copy to prevent external mutation
	result := make([]RememberedChat, len(RememberedChats))
	copy(result, RememberedChats)
	return result
}

func SetRememberedChats(chats []RememberedChat) {
	rememberedChatsMu.Lock()
	defer rememberedChatsMu.Unlock()
	RememberedChats = chats
}

func UpdateRememberedChat(esn string, chat []openai.ChatCompletionMessage) {
	rememberedChatsMu.Lock()
	defer rememberedChatsMu.Unlock()
	for i, achat := range RememberedChats {
		if achat.ESN == esn {
			RememberedChats[i].Chats = chat
			return
		}
	}
	// Not found, append new
	RememberedChats = append(RememberedChats, RememberedChat{
		ESN:   esn,
		Chats: chat,
	})
}

// Thread-safe accessors for RecurringInfo
func GetRecurringInfo() []RecurringInfoStore {
	recurringInfoMu.RLock()
	defer recurringInfoMu.RUnlock()
	result := make([]RecurringInfoStore, len(RecurringInfo))
	copy(result, RecurringInfo)
	return result
}

type RememberedChat struct {
	ESN   string                         `json:"esn"`
	Chats []openai.ChatCompletionMessage `json:"chats"`
}

var RememberedChats []RememberedChat

type RobotEntry struct {
	Esn       string `json:"esn"`
	IPAddress string `json:"ip_address"`
	// 192.168.1.150:443
	GUID      string `json:"guid"`
	Activated bool   `json:"activated"`
}

type RobotInfoStore struct {
	GlobalGUID string       `json:"global_guid"`
	Robots     []RobotEntry `json:"robots"`
}

// GetBotInfo returns a snapshot copy of BotInfo safe for read-only use.
func GetBotInfo() RobotInfoStore {
	botInfoMu.RLock()
	defer botInfoMu.RUnlock()
	cp := BotInfo
	cp.Robots = make([]RobotEntry, len(BotInfo.Robots))
	copy(cp.Robots, BotInfo.Robots)
	return cp
}

// UpdateBotInfo runs fn with exclusive access to BotInfo, then persists it to BotInfoPath.
func UpdateBotInfo(fn func(*RobotInfoStore)) error {
	botInfoMu.Lock()
	defer botInfoMu.Unlock()
	fn(&BotInfo)
	writeBytes, err := json.Marshal(BotInfo)
	if err != nil {
		logger.Println("Error marshaling bot info:", err)
		return err
	}
	if err := fileutil.WriteFileAtomic(BotInfoPath, writeBytes, 0644); err != nil {
		logger.Println("Error writing bot info to", BotInfoPath, ":", err)
		return err
	}
	return nil
}

type RecurringInfoStore struct {
	// Vector-R2D2
	ID string `json:"id"`
	// 00e20145
	ESN string `json:"esn"`
	// 192.168.1.150
	IP string `json:"ip"`
}

type JsonIntent struct {
	Name              string   `json:"name"`
	Keyphrases        []string `json:"keyphrases"`
	RequireExactMatch bool     `json:"requiresexact"`
}

type CustomIntent struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Utterances  []string `json:"utterances"`
	Intent      string   `json:"intent"`
	Params      struct {
		ParamName  string `json:"paramname"`
		ParamValue string `json:"paramvalue"`
	} `json:"params"`
	Exec           string   `json:"exec"`
	ExecArgs       []string `json:"execargs"`
	IsSystemIntent bool     `json:"issystem"`
	LuaScript      string   `json:"luascript"`
}

type AJdoc struct {
	DocVersion     uint64 `protobuf:"varint,1,opt,name=doc_version,json=docVersion,proto3" json:"doc_version,omitempty"`            // first version = 1; 0 => invalid or doesn't exist
	FmtVersion     uint64 `protobuf:"varint,2,opt,name=fmt_version,json=fmtVersion,proto3" json:"fmt_version,omitempty"`            // first version = 1; 0 => invalid
	ClientMetadata string `protobuf:"bytes,3,opt,name=client_metadata,json=clientMetadata,proto3" json:"client_metadata,omitempty"` // arbitrary client-defined string, eg a data fingerprint (typ "", 32 chars max)
	JsonDoc        string `protobuf:"bytes,4,opt,name=json_doc,json=jsonDoc,proto3" json:"json_doc,omitempty"`
}

type botjdoc struct {
	// vic:00000000
	Thing string `json:"thing"`
	// vic.RobotSettings, etc
	Name string `json:"name"`
	// actual jdoc
	Jdoc AJdoc `json:"jdoc"`
}

func join(p1, p2 string) string {
	return filepath.Join(p1, p2)
}

func Init() {
	logger.Println("Commit SHA: " + CommitSHA)
	if VarsInited {
		logger.Println("Not initting vars again")
		return
	}
	logger.Println("Initializing variables")

	if Packaged {
		logger.Println("This version of wire-pod is packaged. Set vars to include UserConfigDir...")
		var confDir string
		if runtime.GOOS == "android" || runtime.GOOS == "ios" {
			confDir = AndroidPath
		} else {
			confDir, _ = os.UserConfigDir()
		}
		podDir := join(confDir, PodName)
		appDir, _ := os.Executable()
		os.Mkdir(podDir, 0777)
		JdocsDir = join(podDir, JdocsDir)
		JdocsPath = JdocsDir + "/jdocs.json"
		CustomIntentsPath = join(podDir, CustomIntentsPath)
		BotInfoPath = JdocsDir + "/" + BotInfoName
		VoskModelPath = join(podDir, "./vosk/models/")
		WhisperModelPath = join(filepath.Dir(appDir), "/../Frameworks/chipper/whisper.cpp/models/") // macos
		ApiConfigPath = join(podDir, ApiConfigPath)
		CertPath = join(podDir, "./certs/cert.crt")
		KeyPath = join(podDir, "./certs/cert.key")
		ServerConfigPath = join(podDir, "./certs/server_config.json")
		Certs = join(podDir, "./certs")
		SessionCertPath = join(podDir, SessionCertPath)
		if runtime.GOOS == "android" {
			VersionFile = AndroidPath + "/static/version"
		}
		os.Mkdir(JdocsDir, 0777)
		os.Mkdir(SessionCertPath, 0777)
		os.Mkdir(Certs, 0777)
	}

	if os.Getenv("WEBSERVER_PORT") != "" {
		if _, err := strconv.Atoi(os.Getenv("WEBSERVER_PORT")); err == nil {
			WebPort = os.Getenv("WEBSERVER_PORT")
		} else {
			logger.Println("WEBSERVER_PORT contains letters, using default of 8080")
			WebPort = "8080"
		}
	} else {
		WebPort = "8080"
	}

	// figure out user SDK path, containing sdk_config.ini
	// has to be done like this because wire-pod is running as root
	// path should be /home/name/wire-pod/chipper
	// Split puts an extra / in the beginning of the array
	podPath, _ := os.Getwd()
	podPathSplit := strings.Split(strings.TrimSpace(podPath), "/")
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		dir, _ := os.UserHomeDir()
		SDKIniPath = dir + "/.anki_vector/"
	} else if runtime.GOOS == "android" || runtime.GOOS == "ios" {
		SDKIniPath = filepath.Join(AndroidPath, "/wire-pod/anki_vector")
	} else {
		if podPathSplit[len(podPathSplit)-1] != "chipper" || podPathSplit[len(podPathSplit)-2] != PodName {
			logger.Println("It looks like you may have changed path names of the directories wire-pod is running in. This is not recommended because the SDK implementation depends on relativity in a few spots.")
		}
		if len(podPathSplit) >= 5 {
			SDKIniPath = "/" + podPathSplit[1] + "/" + podPathSplit[2] + "/.anki_vector/"
		} else if strings.EqualFold(podPathSplit[0], "root") {
			SDKIniPath = "/root/.anki_vector/"
		} else if len(podPathSplit) == 4 {
			SDKIniPath = "/" + podPathSplit[1] + "/.anki_vector/"
		} else {
			logger.Println("Unsupported path scenario, printing podPathSplit: ")
			logger.Println(podPathSplit)
			SDKIniPath = "/tmp/.anki_vector/"
		}
	}
	logger.Println("SDK info path: " + SDKIniPath)

	// load api config (config.go)
	ReadConfig()

	// check models folder, add all models to DownloadedVoskModels
	if APIConfig.STT.Service == "vosk" {
		GetDownloadedVoskModels()
	}

	// load jdocs. if there are any in the old format, conver
	if _, err := os.Stat(JdocsPath); err == nil {
		jsonBytes, err := os.ReadFile(JdocsPath)
		if err != nil {
			logger.Println(fmt.Sprintf("Error reading jdocs file: %v", err))
		} else {
			json.Unmarshal(jsonBytes, &BotJdocs)
			logger.Println("Loaded jdocs file")
		}
	}

	// load bot sdk info
	botBytes, err := os.ReadFile(BotInfoPath)
	if err == nil {
		json.Unmarshal(botBytes, &BotInfo)
		var botList []string
		for _, robot := range BotInfo.Robots {
			botList = append(botList, robot.Esn)
		}
		logger.Println("Loaded bot info file, known bots: " + fmt.Sprint(botList))
	}

	ReadSessionCerts()
	LoadCustomIntents()
	VarsInited = true
}

func GetDownloadedVoskModels() {
	array, err := os.ReadDir(VoskModelPath)
	if err != nil {
		logger.Println(err)
		return
	}
	for _, dir := range array {
		DownloadedVoskModels = append(DownloadedVoskModels, dir.Name())
	}
}

func LoadCustomIntents() {
	jsonBytes, err := os.ReadFile(CustomIntentsPath)
	if err == nil {
		json.Unmarshal(jsonBytes, &CustomIntents)
		CustomIntentsExist = true
		logger.Println("Loaded custom intents:")
		for _, intent := range CustomIntents {
			logger.Println(intent.Name)
		}
	}
}

func LoadIntents() ([]JsonIntent, error) {
	var path string
	if runtime.GOOS == "darwin" && Packaged {
		appPath, _ := os.Executable()
		path = filepath.Dir(appPath) + "/../Frameworks/chipper/"
	} else if runtime.GOOS == "android" || runtime.GOOS == "ios" {
		path = AndroidPath + "/static/"
	} else {
		path = "./"
	}
	jsonFile, err := os.ReadFile(path + "intent-data/" + APIConfig.STT.Language + ".json")

	// var matches [][]string
	// var intents []string
	var jsonIntents []JsonIntent
	if err == nil {
		err = json.Unmarshal(jsonFile, &jsonIntents)
		// if err != nil {
		// 	logger.Println("Failed to load intents: " + err.Error())
		// }

		// for _, element := range jsonIntents {
		// 	//logger.Println("Loading intent " + strconv.Itoa(index) + " --> " + element.Name + "( " + strconv.Itoa(len(element.Keyphrases)) + " keyphrases )")
		// 	intents = append(intents, element.Name)
		// 	matches = append(matches, element.Keyphrases)
		// }
		// logger.Println("Loaded " + strconv.Itoa(len(jsonIntents)) + " intents and " + strconv.Itoa(len(matches)) + " matches (language: " + APIConfig.STT.Language + ")")
	}
	return jsonIntents, err
}

// writeJdocsLocked writes BotJdocs to disk. Caller must hold botJdocsMu.
// writeJdocsLocked persists BotJdocs atomically. Caller must hold botJdocsMu.
// Failures are logged here rather than silently dropped, since none of the
// exported functions that call this (WriteJdocs, DeleteData, AddJdoc) return
// an error today.
func writeJdocsLocked() {
	writeBytes, err := json.Marshal(BotJdocs)
	if err != nil {
		logger.Println("Error marshaling jdocs:", err)
		return
	}
	if err := fileutil.WriteFileAtomic(JdocsPath, writeBytes, 0644); err != nil {
		logger.Println("Error writing jdocs to", JdocsPath, ":", err)
	}
}

func WriteJdocs() {
	botJdocsMu.Lock()
	defer botJdocsMu.Unlock()
	writeJdocsLocked()
}

// removes a bot from jdocs file
func DeleteData(thing string) {
	botJdocsMu.Lock()
	defer botJdocsMu.Unlock()
	var newdocs []botjdoc
	for _, jdocentry := range BotJdocs {
		if jdocentry.Thing != thing {
			newdocs = append(newdocs, jdocentry)
		}
	}
	BotJdocs = newdocs
	writeJdocsLocked()
}

func GetJdoc(thing, jdocname string) (AJdoc, bool) {
	botJdocsMu.RLock()
	defer botJdocsMu.RUnlock()
	for _, botJdoc := range BotJdocs {
		if botJdoc.Name == jdocname && botJdoc.Thing == thing {
			return botJdoc.Jdoc, true
		}
	}
	return AJdoc{}, false
}

//    DocVersion     uint64 `protobuf:"varint,1,opt,name=doc_version,json=docVersion,proto3" json:"doc_version,omitempty"`            // first version = 1; 0 => invalid or doesn't exist
// FmtVersion     uint64 `protobuf:"varint,2,opt,name=fmt_version,json=fmtVersion,proto3" json:"fmt_version,omitempty"`            // first version = 1; 0 => invalid
// ClientMetadata string `protobuf:"bytes,3,opt,name=client_metadata,json=clientMetadata,proto3" json:"client_metadata,omitempty"` // arbitrary client-defined string, eg a data fingerprint (typ "", 32 chars max)
// JsonDoc        string

func AddJdoc(thing string, name string, jdoc AJdoc) uint64 {
	botJdocsMu.Lock()
	defer botJdocsMu.Unlock()
	var latestVersion uint64 = 0
	matched := false
	for index, jdocentry := range BotJdocs {
		if jdocentry.Thing == thing && jdocentry.Name == name {
			BotJdocs[index].Jdoc = jdoc
			latestVersion = BotJdocs[index].Jdoc.DocVersion
			matched = true
			break
		}
	}
	if !matched {
		var newbot botjdoc
		newbot.Thing = thing
		newbot.Name = name
		newbot.Jdoc = jdoc
		BotJdocs = append(BotJdocs, newbot)
	}
	writeJdocsLocked()
	return latestVersion
}

func ReadSessionCerts() {
	logger.Println("Reading session certs for robot IDs")
	var rinfo RecurringInfoStore
	certDir, err := os.ReadDir(SessionCertPath)
	if err != nil {
		logger.Println(err)
		return
	}
	recurringInfoMu.Lock()
	defer recurringInfoMu.Unlock()
	for _, entry := range certDir {
		if entry.Name() == "placeholder" {
			continue
		}
		esn := entry.Name()
		var ip string
		certBytes, err := os.ReadFile(filepath.Join(SessionCertPath, entry.Name()))
		if err != nil {
			logger.Println(err)
			return
		}
		pemBytes, _ := pem.Decode(certBytes)
		cert, _ := x509.ParseCertificate(pemBytes.Bytes)
		for _, robot := range BotInfo.Robots {
			if esn == robot.Esn {
				ip = robot.IPAddress
				break
			}
		}
		rinfo.ESN = esn
		rinfo.ID = cert.Issuer.CommonName
		rinfo.IP = ip
		RecurringInfo = append(RecurringInfo, rinfo)
	}
}

func AddToRInfo(esn string, id string, ip string) {
	recurringInfoMu.Lock()
	defer recurringInfoMu.Unlock()
	// the only bot constant is ESN
	for i := range RecurringInfo {
		if RecurringInfo[i].ESN == esn {
			RecurringInfo[i].ID = id
			RecurringInfo[i].IP = ip
			return
		}
	}
	var rinfo RecurringInfoStore
	rinfo.ESN = esn
	rinfo.ID = id
	rinfo.IP = ip
	RecurringInfo = append(RecurringInfo, rinfo)
}

func GetRobot(esn string) (*vector.Vector, error) {
	var guid string
	var target string
	matched := false
	for _, bot := range BotInfo.Robots {
		if esn == bot.Esn {
			guid = bot.GUID
			target = bot.IPAddress + ":443"
			matched = true
			break
		}
	}
	if !matched {
		return nil, errors.New("robot not in botsdkinfo")
	}
	robot, err := vector.New(vector.WithSerialNo(esn), vector.WithToken(guid), vector.WithTarget(target))
	if err != nil {
		return nil, err
	}
	return robot, nil
}

func GetOutboundIP() net.IP {
	if runtime.GOOS == "android" {
		ifaces, _ := anet.Interfaces()
		for _, iface := range ifaces {
			if iface.Name == "wlan0" {
				adrs, err := anet.InterfaceAddrsByInterface(&iface)
				if err != nil {
					logger.Println(err)
					break
				}
				if len(adrs) > 0 {
					localAddr := adrs[0].(*net.IPNet)
					return localAddr.IP
				}
			}
		}
	}
	conn, err := net.Dial("udp", OutboundIPTester)
	if err != nil {
		logger.Println("not connected to a network: ", err)
		return net.IPv4(0, 0, 0, 0)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}
