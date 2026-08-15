function updateSetupStatus(statusString) {
  const setupStatus = document.getElementById("setup-status");
  setupStatus.innerHTML = `<p>${statusString}</p>`;
}

// STT service/language selection now lives entirely in Server Settings
// (setup.html), same as weather/Knowledge Graph. First-run just needs
// *a* model in place so wire-pod can function immediately after pairing,
// so this silently defaults to en-US for the vosk/whisper.cpp backends --
// change it (or the service itself, or point Whisper at a custom
// endpoint) from Server Settings afterward. Any other backend has
// nothing to set up here at all.
function sendSetupInfo() {
  document.getElementById("config-options").style.display = "none";
  updateSetupStatus("Initiating setup...");

  fetch("/api/get_stt_info")
    .then((response) => response.json())
    .then((parsed) => {
      if (parsed.provider !== "vosk" && parsed.provider !== "whisper.cpp") {
        setConn();
        return;
      }

      fetch("/api/set_stt_info", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ language: "en-US" }),
      })
        .then((response) => response.text())
        .then((response) => {
          if (response.includes("success")) {
            updateSetupStatus("Language set successfully.");
            setConn();
          } else if (response.includes("downloading")) {
            updateSetupStatus("Downloading language model...");
            var interval = setInterval(() => {
              fetch("/api/get_download_status")
                .then((response) => response.text())
                .then((statusText) => {
                  updateSetupStatus(statusText);
                  if (statusText.includes("success")) {
                    updateSetupStatus("Language set successfully.");
                    clearInterval(interval);
                    setConn();
                  } else if (statusText.includes("error")) {
                    document.getElementById("config-options").style.display = "block";
                    clearInterval(interval);
                  } else if (statusText.includes("not downloading")) {
                    updateSetupStatus("Initiating language model download...");
                  }
                });
            }, 500);
          } else {
            updateSetupStatus(response);
            document.getElementById("config-options").style.display = "block";
          }
        });
    });
}

function checkConn() {
  const connValue = document.getElementById("connSelection").value;
  document.getElementById("portViz").style.display = connValue === "ip" || connValue === "custom" ? "block" : "none";
  document.getElementById("hostViz").style.display = connValue === "custom" ? "block" : "none";
}

function setConn() {
  updateSetupStatus("Setting connection type (ep, ip, or custom host)...");
  const connValue = document.getElementById("connSelection").value;
  let port = document.getElementById("portInput").value;
  port = port ? port : "443";

  let url;
  if (connValue === "ep") {
    url = "/api-chipper/use_ep";
  } else if (connValue === "custom") {
    const host = document.getElementById("hostInput").value.trim();
    if (!host) {
      updateSetupStatus("Error: a custom host requires a domain or IP address.");
      document.getElementById("config-options").style.display = "block";
      return;
    }
    url = `/api-chipper/use_custom?host=${encodeURIComponent(host)}&port=${encodeURIComponent(port)}`;
  } else {
    url = `/api-chipper/use_ip?port=${port}`;
  }

  fetch(url)
    .then((response) => response.text())
    .then((response) => {
      if (response && response === "done") {
        updateSetupStatus("Setup is complete! Wire-pod has started. Redirecting to main page...");
        setTimeout(() => window.location.href = "/", 3000);
      } else {
        updateSetupStatus(response || "Error setting up wire-pod, check the logs");
        document.getElementById("config-options").style.display = "block";
      }
    });
}

function directToIndex() {
  window.location.href = "/index.html";
}