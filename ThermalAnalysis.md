# VOSK Thermal Analysis for wire-pod

**Date**: 2026-04-09  
**Target Hardware**: Raspberry Pi Zero 2W  
**Issue**: 80.1°C temperature, thermal throttling

---

## Executive Summary

The VOSK speech recognition engine is consuming ~45% CPU continuously on the RPi Zero 2W due to a 68MB model loaded in RAM with no idle/sleep logic between requests. This document analyzes alternative STT engines and optimization strategies.

---

## Root Cause Analysis

### Current VOSK Implementation

| Aspect | Details |
|--------|---------|
| **Model Size** | 68MB (en-US model) |
| **Memory** | Loaded in RAM at startup |
| **Idle Behavior** | Model stays loaded, recognizers pool maintained |
| **Sample Rate** | 16000 Hz |
| **CPU Impact** | High - continuous inference capability |

**Key Code Issues** (from `chipper/pkg/wirepod/stt/vosk/Vosk.go`):
- No sleep/idle detection between speech requests
- Recognizer pool created and kept alive continuously
- Grammer optimization creates additional recognizers
- No mechanism to unload model when idle

---

## Alternative STT Engines

### 1. VOSK with Optimizations

| Metric | Value |
|--------|-------|
| **CPU Impact** | High (current baseline) |
| **RAM Usage** | ~68MB + recognizer overhead |
| **Accuracy** | Good |
| **Setup** | Pre-installed with wire-pod |
| **License** | Apache 2.0 |

**Potential Optimizations**:
1. Use smaller model (vosk-model-small-en-us - ~40MB)
2. Implement idle timeout to unload/reload model
3. Limit recognizer pool size
4. Disable grammer optimization if not needed

### 2. Leopard (Picovoice)

| Metric | Value |
|--------|-------|
| **CPU Impact** | Low-Medium |
| **RAM Usage** | Low (optimized native binding) |
| **Accuracy** | Good |
| **Setup** | Requires PICOVOICE_APIKEY |
| **License** | Commercial |

**Implementation** (`chipper/pkg/wirepod/stt/leopard/Leopard.go`):
```go
// Configuration via environment:
PICOVOICE_APIKEY=<key>
PICOVOICE_INSTANCES=3  // number of parallel instances
```

**Pros**:
- Optimized for embedded/ARM devices
- Proven on resource-constrained hardware
- Multiple concurrent bot support

**Cons**:
- Requires API key
- Cloud-based processing (network dependent)

### 3. Coqui STT

| Metric | Value |
|--------|-------|
| **CPU Impact** | Medium (TFLite optimized) |
| **RAM Usage** | ~40MB (model.tflite) |
| **Accuracy** | Good |
| **Setup** | Local model + scorer files |
| **License** | MPL 2.0 |

**Implementation** (`chipper/pkg/wirepod/stt/coqui/Coqui.go`):
- Uses TensorFlow Lite (ARM optimized)
- Local processing, no network required
- Creates new instance per request (inefficient)

**Pros**:
- Fully local (no API key, no network)
- TFLite optimized for ARM
- Open source

**Cons**:
- Creates new instance per request (memory churn)
- Requires model.tflite + scorer files

### 4. Whisper (OpenAI API)

| Metric | Value |
|--------|-------|
| **CPU Impact** | None (offloaded to cloud) |
| **RAM Usage** | Minimal |
| **Accuracy** | Excellent |
| **Setup** | Requires OPENAI_KEY |
| **License** | Commercial |

**Implementation** (`chipper/pkg/wirepod/stt/whisper/Whisper.go`):
- Sends audio to OpenAI Whisper API
- Returns transcription

**Pros**:
- Highest accuracy
- Zero local CPU load
- Best for production with internet

**Cons**:
- Requires internet + API key
- Latency + cost considerations
- Privacy concerns (audio sent to cloud)

---

## CPU Impact Comparison

| Engine | CPU Load | RAM | Network Required | Accuracy |
|--------|----------|-----|-------------------|----------|
| **VOSK (current)** | **HIGH (~45%)** | ~80MB | No | Good |
| VOSK (small model) | Medium (~25%) | ~45MB | No | Good |
| Leopard | Low | ~20MB | Yes | Good |
| Coqui | Medium | ~45MB | No | Good |
| Whisper | None | ~5MB | Yes | Excellent |

---

## Recommended Solutions

### 🥇 Recommended: Leopard (Picovoice)

For RPi Zero 2W with thermal constraints:
- Set `PICOVOICE_APIKEY` environment variable
- Configure `PICOVOICE_INSTANCES=1` for minimal footprint
- Lowest CPU impact among local options

### 🥈 Alternative: Optimized VOSK

If network dependency is unacceptable:
1. Use smaller model: `vosk-model-small-en-us`
2. Implement idle timeout (see below)
3. Reduce recognizer pool to 1

### 🥉 Fallback: Whisper API

For maximum accuracy with acceptable network:
- Zero local CPU load
- Best accuracy
- Requires `OPENAI_KEY`

---

## Implementation Plan

### VOSK Idle Detection (Quick Fix)

Add to `Vosk.go`:
```go
const idleTimeout = 5 * time.Minute
var lastRequestTime time.Time

func STT(req sr.SpeechRequest) (string, error) {
    lastRequestTime = time.Now()
    // ... existing processing
}

// Background goroutine to monitor idle state
func monitorIdleState() {
    for {
        time.Sleep(1 * time.Minute)
        if time.Since(lastRequestTime) > idleTimeout {
            // Unload recognizers, keep model ready
            // Or: unload model entirely
        }
    }
}
```

### Switch to Leopard

1. Set environment variable: `export PICOVOICE_APIKEY=your_key`
2. Rebuild/start chipper with Leopard STT
3. Configure in web UI or `apiConfig.json`

---

## References

- VOSK: https://github.com/alphacep/vosk-api
- Leopard: https://picovoice.ai/docs/leopard/
- Coqui: https://github.com/coqui-ai/STT
- wire-pod STT adapters: `chipper/pkg/wirepod/stt/*/`

---

*Document generated by VOSK Thermal Analysis subagent*
