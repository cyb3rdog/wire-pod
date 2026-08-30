const intentsJson = JSON.parse(
  '["intent_greeting_hello", "intent_names_ask", "intent_imperative_eyecolor", "intent_character_age", "intent_explore_start", "intent_system_charger", "intent_system_sleep", "intent_greeting_goodmorning", "intent_greeting_goodnight", "intent_greeting_goodbye", "intent_seasonal_happynewyear", "intent_seasonal_happyholidays", "intent_amazon_signin", "intent_imperative_forward", "intent_imperative_turnaround", "intent_imperative_turnleft", "intent_imperative_turnright", "intent_play_rollcube", "intent_play_popawheelie", "intent_play_fistbump", "intent_play_blackjack", "intent_imperative_affirmative", "intent_imperative_negative", "intent_photo_take_extend", "intent_imperative_praise", "intent_imperative_abuse", "intent_weather_extend", "intent_imperative_apologize", "intent_imperative_backup", "intent_imperative_volumedown", "intent_imperative_volumeup", "intent_imperative_lookatme", "intent_imperative_volumelevel_extend", "intent_imperative_shutup", "intent_names_username_extend", "intent_imperative_come", "intent_imperative_love", "intent_knowledge_promptquestion", "intent_clock_checktimer", "intent_global_stop_extend", "intent_clock_settimer_extend", "intent_clock_time", "intent_imperative_quiet", "intent_imperative_dance", "intent_play_pickupcube", "intent_imperative_fetchcube", "intent_imperative_findcube", "intent_play_anytrick", "intent_message_recordmessage_extend", "intent_message_playmessage_extend", "intent_blackjack_hit", "intent_blackjack_stand", "intent_play_keepaway"]'
);

var GetLog = false;

const getE = (element) => document.getElementById(element);

// postJSON/getJSON: the fetch-with-JSON-headers-then-read-the-body
// boilerplate below used to be copy-pasted into every settings
// send/update function in this file (and initial.js, which loads after
// this file on initial.html and shares its global scope) -- 8+
// near-identical copies. Both return the parsed response as a Promise,
// same as calling fetch(...).then(...) directly would, so existing
// .then()/.catch() chains at call sites don't need to change shape.
function postJSON(url, data) {
  return fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  }).then((response) => response.text());
}

function getJSON(url) {
  return fetch(url).then((response) => response.json());
}

function updateIntentSelection(element) {
  getJSON("/api/get_custom_intents_json")
    .then((listResponse) => {
      const container = getE(element);
      container.innerHTML = "";
      if (listResponse && listResponse.length > 0) {
        const select = document.createElement("select");
        select.name = `${element}intents`;
        select.id = `${element}intents`;
        listResponse.forEach((intent) => {
          if (!intent.issystem) {
            const option = document.createElement("option");
            option.value = intent.name;
            option.text = intent.name;
            select.appendChild(option);
          }
        });
        const label = document.createElement("label");
        label.innerHTML = "Choose the intent: ";
        label.htmlFor = `${element}intents`;
        container.appendChild(label).appendChild(select);

        select.addEventListener("change", hideEditIntents);
      } else {
        const error = document.createElement("p");
        error.innerHTML = "No intents found, you must add one first";
        container.appendChild(error);
      }
    }).catch(() => {
      // Do nothing
    });
}

function checkInited() {
  fetch("/api/is_api_v3").then((response) => {
    if (!response.ok) {
      alert(
        "This webroot does not match with the wire-pod binary. Some functionality will be broken. There was either an error during the last update, or you did not precisely follow the update guide. https://github.com/kercre123/wire-pod/wiki/Things-to-Know#updating-wire-pod"
      );
    }
  });

  getJSON("/api/get_config").then((config) => {
    if (!config.pastinitialsetup) {
      window.location.href = "/initial.html";
    }
  });
}

function createIntentSelect(element) {
  const select = document.createElement("select");
  select.name = `${element}intents`;
  select.id = `${element}intents`;
  intentsJson.forEach((intent) => {
    const option = document.createElement("option");
    option.value = intent;
    option.text = intent;
    select.appendChild(option);
  });
  const label = document.createElement("label");
  label.innerHTML = "Intent to send to robot after script executed:";
  label.htmlFor = `${element}intents`;
  getE(element).innerHTML = "";
  getE(element).appendChild(label).appendChild(select);
}

function editFormCreate() {
  const intentNumber = getE("editSelectintents").selectedIndex;

  getJSON("/api/get_custom_intents_json")
    .then((intents) => {
      const intent = intents[intentNumber];
      if (intent) {
        const form = document.createElement("form");
        form.id = "editIntentForm";
        form.name = "editIntentForm";
        form.innerHTML = `
          <label for="name">Name:<br><input type="text" id="name" value="${intent.name}"></label><br>
          <label for="description">Description:<br><input type="text" id="description" value="${intent.description}"></label><br>
          <label for="utterances">Utterances:<br><input type="text" id="utterances" value="${intent.utterances.join(",")}"></label><br>
          <label for="intent">Intent:<br><select id="intent">${intentsJson
            .map(
              (name) =>
                `<option value="${name}" ${name === intent.intent ? "selected" : ""
                }>${name}</option>`
            )
            .join("")}</select></label><br>
          <label for="paramname">Param Name:<br><input type="text" id="paramname" value="${intent.params.paramname}"></label><br>
          <label for="paramvalue">Param Value:<br><input type="text" id="paramvalue" value="${intent.params.paramvalue}"></label><br>
          <label for="exec">Exec:<br><input type="text" id="exec" value="${intent.exec}"></label><br>
          <label for="execargs">Exec Args:<br><input type="text" id="execargs" value="${intent.execargs.join(",")}"></label><br>
          <label for="luascript">Lua code to run:</label><br><textarea id="luascript">${intent.luascript}</textarea>
          <button onclick="editIntent(${intentNumber})">Submit</button>
        `;
        //form.querySelector("#submit").onclick = () => editIntent(intentNumber);
        getE("editIntentForm").innerHTML = "";
        getE("editIntentForm").appendChild(form);
        showEditIntents();
      } else {
        displayError("editIntentForm", "No intents found, you must add one first");
      }
    }).catch((error) => {
      console.error(error);
      displayError("editIntentForm", "Error fetching intents");
    })
}

function editIntent(intentNumber) {
  const data = {
    number: intentNumber + 1,
    name: getE("name").value,
    description: getE("description").value,
    utterances: getE("utterances").value.split(","),
    intent: getE("intent").value,
    params: {
      paramname: getE("paramname").value,
      paramvalue: getE("paramvalue").value,
    },
    exec: getE("exec").value,
    execargs: getE("execargs").value.split(","),
    luascript: getE("luascript").value,
  };

  postJSON("/api/edit_custom_intent", data).then((response) => {
    displayMessage("editIntentStatus", response);
    alert(response)
    updateIntentSelection("editSelect");
    updateIntentSelection("deleteSelect");
  });
}

function deleteSelectedIntent() {
  const intentNumber = getE("editSelectintents").selectedIndex + 1;

  postJSON("/api/remove_custom_intent", { number: intentNumber }).then((response) => {
    hideEditIntents();
    alert(response)
    updateIntentSelection("editSelect");
    updateIntentSelection("deleteSelect");
  });
}

function sendIntentAdd() {
  const form = getE("intentAddForm");
  const data = {
    name: form.elements["nameAdd"].value,
    description: form.elements["descriptionAdd"].value,
    utterances: form.elements["utterancesAdd"].value.split(","),
    intent: form.elements["intentAddSelectintents"].value,
    params: {
      paramname: form.elements["paramnameAdd"].value,
      paramvalue: form.elements["paramvalueAdd"].value,
    },
    exec: form.elements["execAdd"].value,
    execargs: form.elements["execAddArgs"].value.split(","),
    luascript: form.elements["luaAdd"].value,
  };
  if (!data.name || !data.description || !data.utterances) {
    displayMessage("addIntentStatus", "A required input is missing. You need a name, description, and utterances.");
    alert("A required input is missing. You need a name, description, and utterances.")
    return
  }

  displayMessage("addIntentStatus", "Adding...");

  postJSON("/api/add_custom_intent", data).then((response) => {
    displayMessage("addIntentStatus", response);
    alert(response)
    updateIntentSelection("editSelect");
    updateIntentSelection("deleteSelect");
  });
}

function checkWeather() {
  getE("apiKeySpan").style.display = getE("weatherProvider").value ? "block" : "none";
}

function sendWeatherAPIKey() {
  const data = {
    provider: getE("weatherProvider").value,
    key: getE("apiKey").value,
  };

  displayMessage("addWeatherProviderAPIStatus", "Saving...");

  postJSON("/api/set_weather_api", data).then((response) => {
    displayMessage("addWeatherProviderAPIStatus", response);
  });
}

function updateWeatherAPI() {
  getJSON("/api/get_weather_api").then((data) => {
    getE("weatherProvider").value = data.provider;
    getE("apiKey").value = data.key;
    checkWeather();
  });
}

// KG_PROVIDER_CONTAINERS: every container div checkKG toggles, always
// hidden first regardless of the selected provider.
const KG_PROVIDER_CONTAINERS = [
  "houndifyInput",
  "togetherInput",
  "customAIInput",
  "intentGraphInput",
  "openAIInput",
  "saveChatInput",
  "llmCommandInput",
  "openAIVoiceForEnglishInput",
];

// KG_PROVIDERS is the one place the "Ask" (knowledge graph) feature's
// per-provider shape is declared -- which input containers a provider
// shows (checkKG), and which form fields map to which request/response
// JSON keys, value vs. checkbox (sendKGAPIKey writes these out,
// updateKGAPI reads them back the same way). Previously this same
// provider list, and its per-field mapping, was hand-duplicated across
// all three functions -- easy for a provider's fields to drift out of
// sync between what gets saved and what gets displayed on reload, since
// nothing forced the three copies to agree.
const KG_PROVIDERS = {
  houndify: {
    containers: ["houndifyInput", "intentGraphInput"],
    fields: [
      { el: "houndKey", key: "key" },
      { el: "houndID", key: "id" },
      { el: "intentyes", key: "intentgraph", checkbox: true },
    ],
  },
  openai: {
    containers: [
      "intentGraphInput",
      "openAIInput",
      "saveChatInput",
      "llmCommandInput",
      "openAIVoiceForEnglishInput",
    ],
    fields: [
      { el: "openaiKey", key: "key" },
      { el: "openAIPrompt", key: "openai_prompt" },
      { el: "intentyes", key: "intentgraph", checkbox: true },
      { el: "saveChatYes", key: "save_chat", checkbox: true },
      { el: "commandYes", key: "commands_enable", checkbox: true },
      { el: "openaiVoice", key: "openai_voice" },
      { el: "voiceEnglishYes", key: "openai_voice_with_english", checkbox: true },
    ],
  },
  together: {
    containers: ["intentGraphInput", "togetherInput", "saveChatInput", "llmCommandInput"],
    fields: [
      { el: "togetherKey", key: "key" },
      { el: "togetherModel", key: "model" },
      { el: "togetherAIPrompt", key: "openai_prompt" },
      { el: "intentyes", key: "intentgraph", checkbox: true },
      { el: "saveChatYes", key: "save_chat", checkbox: true },
      { el: "commandYes", key: "commands_enable", checkbox: true },
    ],
  },
  custom: {
    containers: ["intentGraphInput", "customAIInput", "saveChatInput", "llmCommandInput"],
    fields: [
      { el: "customKey", key: "key" },
      { el: "customModel", key: "model" },
      { el: "customAIPrompt", key: "openai_prompt" },
      { el: "customAIEndpoint", key: "endpoint" },
      { el: "intentyes", key: "intentgraph", checkbox: true },
      { el: "saveChatYes", key: "save_chat", checkbox: true },
      { el: "commandYes", key: "commands_enable", checkbox: true },
    ],
  },
};

function checkKG() {
  const provider = getE("kgProvider").value;
  KG_PROVIDER_CONTAINERS.forEach((el) => (getE(el).style.display = "none"));
  const cfg = KG_PROVIDERS[provider];
  if (cfg) {
    cfg.containers.forEach((el) => (getE(el).style.display = "block"));
  }
}

function sendKGAPIKey() {
  const provider = getE("kgProvider").value;
  const data = {
    enable: true,
    provider,
    key: "",
    model: "",
    id: "",
    intentgraph: false,
    robotName: "",
    openai_prompt: "",
    openai_voice: "",
    openai_voice_with_english: false,
    save_chat: false,
    commands_enable: false,
    endpoint: "",
  };
  const cfg = KG_PROVIDERS[provider];
  if (cfg) {
    cfg.fields.forEach(({ el, key, checkbox }) => {
      data[key] = checkbox ? getE(el).checked : getE(el).value;
    });
  } else {
    data.enable = false;
  }

  postJSON("/api/set_kg_api", data).then((response) => {
    displayMessage("addKGProviderAPIStatus", response);
    alert(response);
  });
}

function deleteSavedChats() {
  if (confirm("Are you sure? This will delete all saved chats.")) {
    fetch("/api/delete_chats")
      .then((response) => response.text())
      .then(() => {
        alert("Successfully deleted all saved chats.");
      });
  }
}

function updateKGAPI() {
  getJSON("/api/get_kg_api").then((data) => {
    getE("kgProvider").value = data.provider;
    const cfg = KG_PROVIDERS[data.provider];
    if (cfg) {
      cfg.fields.forEach(({ el, key, checkbox }) => {
        if (checkbox) {
          getE(el).checked = data[key];
        } else {
          getE(el).value = data[key];
        }
      });
    }
    checkKG();
  });
}

function setSTTLanguage() {
  const data = { language: getE("languageSelection").value };

  displayMessage("languageStatus", "Setting...");

  postJSON("/api/set_stt_info", data).then((response) => {
    if (response.includes("downloading")) {
      displayMessage("languageStatus", "Downloading model...");
      updateSTTLanguageDownload();
    } else {
      displayMessage("languageStatus", response);
      getE("languageSelectionDiv").style.display = response.includes("success") ? "block" : "none";
    }
  });
}

function updateSTTLanguageDownload() {

  const interval = setInterval(() => {
    fetch("/api/get_download_status")
      .then((response) => response.text())
      .then((response) => {
        displayMessage("languageStatus", response.includes("not downloading") ? "Initiating download..." : response)
        if (response.includes("success") || response.includes("error")) {
          displayMessage("languageStatus", response);
          getE("languageSelectionDiv").style.display = "block";
          clearInterval(interval);
        }
      });
  }, 500);
}

function checkSTTService() {
  const service = getE("sttServiceSelect").value;
  getE("whisperServiceInput").style.display = service === "whisper" ? "block" : "none";
}

function sendSTTServiceConfig() {
  const data = {
    service: getE("sttServiceSelect").value,
    whisperURL: getE("whisperURL").value,
    whisperKey: getE("whisperKey").value,
    whisperModel: getE("whisperModel").value,
  };

  postJSON("/api/set_stt_service", data).then((response) => {
    displayMessage("addSTTServiceAPIStatus", response);
    alert(response);
  });
}

function updateSTTServiceInfo() {
  getJSON("/api/get_stt_info").then((data) => {
    getE("sttServiceSelect").value = data.provider === "whisper" ? "whisper" : "vosk";
    if (data.whisper) {
      getE("whisperURL").value = data.whisper.base_url || "";
      getE("whisperKey").value = data.whisper.api_key || "";
      getE("whisperModel").value = data.whisper.model || "";
    }
    checkSTTService();
  });
}

function sendRestart() {
  fetch("/api/reset")
    .then((response) => response.text())
    .then((response) => {
      displayMessage("restartStatus", response);
    });
}

function hideEditIntents() {
  getE("editIntentForm").style.display = "none";
  getE("editIntentStatus").innerHTML = "";
}

function showEditIntents() {
  getE("editIntentForm").style.display = "block";
}

function displayMessage(elementId, message) {
  const element = getE(elementId);
  element.innerHTML = "";
  const p = document.createElement("p");
  p.textContent = message;
  element.appendChild(p);
}

function displayError(elementId, message) {
  const element = getE(elementId);
  element.innerHTML = "";
  const error = document.createElement("p");
  error.innerHTML = message;
  element.appendChild(error);
}

function toggleSection(sectionToToggle, sectionToClose, foldableID) {
  const toggleSect = getE(sectionToToggle);
  const closeSect = getE(sectionToClose);

  if (toggleSect.style.display === "block") {
    closeSection(toggleSect, foldableID);
  } else {
    openSection(toggleSect, foldableID);
    closeSection(closeSect, foldableID);
  }
}

function openSection(sectionID) {
  sectionID.style.display = "block";
}

function closeSection(sectionID) {
  sectionID.style.display = "none";
}

function updateColor(id) {
  const l_id = id.replace("section", "icon");
  const elements = document.getElementsByName("icon");

  elements.forEach((element) => {
    element.classList.remove("selectedicon");
    element.classList.add("nowselectedicon");
  });

  const targetElement = document.getElementById(l_id);
  targetElement.classList.remove("notselectedicon");
  targetElement.classList.add("selectedicon");
}


function showLog() {
  toggleVisibility(["section-intents", "section-log", "section-botauth", "section-version", "section-uicustomizer"], "section-log", "icon-Logs");
  logDivArea = getE("botTranscriptedTextArea");
  getE("logscrollbottom").checked = true;
  logP = document.createElement("p");
  GetLog = true
  const interval = setInterval(() => {
    if (!GetLog) {
      clearInterval(interval);
      return;
    }
    const url = getE("logdebug").checked ? "/api/get_debug_logs" : "/api/get_logs";
    fetch(url)
      .then((response) => response.text())
      .then((logs) => {
        logDivArea.innerHTML = logs || "No logs yet, you must say a command to Vector. (this updates automatically)";
        if (getE("logscrollbottom").checked) {
          logDivArea.scrollTop = logDivArea.scrollHeight;
        }
      });
  }, 500);
}

function checkUpdate() {
  displayMessage("cVersion", "Checking for updates...");
  displayMessage("aUpdate", "");
  displayMessage("cCommit", "");
  fetch("/api/get_version_info")
    // type VersionInfo struct {
    // 	FromSource      bool   `json:"fromsource"`
    // 	InstalledVer    string `json:"installedversion"`
    // 	InstalledCommit string `json:"installedcommit"`
    // 	CurrentVer      string `json:"currentver"`
    // 	CurrentCommit   string `json:"currentcommit"`
    // 	UpdateAvailable bool   `json:"avail"`
    // }
    .then((response) => response.text())
    .then((response) => {
      if (response.includes("error")) {
        // <p id="cVersion"></p>
        // <p style="display: none;" id="cCommit"></p>
        // <p id="aUpdate"></p>
        displayMessage(
          "cVersion",
          "There was an error: " + response
        );
        getE("updateGuideLink").style.display = "none";
      } else {
        const parsed = JSON.parse(response);
        if (parsed.fromsource) {
          if (!parsed.avail) {
            displayMessage("aUpdate", `You are on the latest version.`);
            getE("updateGuideLink").style.display = "none";
          } else {
            displayMessage("aUpdate", `A newer version of WirePod (commit: ${parsed.currentcommit}) is available! Use this guide to update WirePod: `);
            getE("updateGuideLink").style.display = "block";
          }
          displayMessage("cVersion", `Installed Commit: ${parsed.installedcommit}`);
        } else {
          displayMessage("cVersion", `Installed Version: ${parsed.installedversion}`);
          displayMessage("cCommit", `Based on wire-pod commit: ${parsed.installedcommit}`);
          getE("cCommit").style.display = "block";
          if (parsed.avail) {
            displayMessage("aUpdate", `A newer version of WirePod (${parsed.currentversion}) is available! Use this guide to update WirePod: `);
            getE("updateGuideLink").style.display = "block";
          } else {
            displayMessage("aUpdate", "You are on the latest version.");
            getE("updateGuideLink").style.display = "none";
          }
        }
      }
    });
}

function showLanguage() {
  toggleVisibility(["section-weather", "section-restart", "section-kg", "section-language", "section-stt-service"], "section-language", "icon-Language");
  getJSON("/api/get_stt_info").then((parsed) => {
    if (parsed.provider !== "vosk" && parsed.provider !== "whisper.cpp") {
      displayError("languageStatus", `To set the STT language, the provider must be Vosk or Whisper. The current one is '${parsed.sttProvider}'.`);
      getE("languageSelectionDiv").style.display = "none";
    } else {
      getE("languageSelectionDiv").style.display = "block";
      getE("languageSelection").value = parsed.language;
    }
  });
}

function showVersion() {
  toggleVisibility(["section-log", "section-botauth", "section-intents", "section-version", "section-uicustomizer"], "section-version", "icon-Version");
  checkUpdate();
}

function showIntents() {
  toggleVisibility(["section-log", "section-botauth", "section-intents", "section-version", "section-uicustomizer"], "section-intents", "icon-Intents");
}

function showWeather() {
  toggleVisibility(["section-weather", "section-restart", "section-language", "section-kg", "section-stt-service"], "section-weather", "icon-Weather");
}

function showKG() {
  toggleVisibility(["section-weather", "section-restart", "section-language", "section-kg", "section-stt-service"], "section-kg", "icon-KG");
}

function showSTTService() {
  toggleVisibility(["section-weather", "section-restart", "section-language", "section-kg", "section-stt-service"], "section-stt-service", "icon-STTService");
}

function toggleVisibility(sections, sectionToShow, iconId) {
  if (sectionToShow != "section-log") {
    GetLog = false;
  }
  sections.forEach((section) => {
    getE(section).style.display = "none";
  });
  getE(sectionToShow).style.display = "block";
  updateColor(iconId);
}