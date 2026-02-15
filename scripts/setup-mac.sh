#!/usr/bin/env bash
set -euo pipefail

# PiBuddy — macOS setup script
# Run this on your Mac to install dependencies, download models, and build.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PIBUDDY_DIR="${PIBUDDY_DIR:-$(dirname "$SCRIPT_DIR")}"
MODELS_DIR="${PIBUDDY_DIR}/models"

echo "=== PiBuddy macOS Setup ==="
echo "Project dir: ${PIBUDDY_DIR}"

# --- Check prerequisites ---
echo ""
echo "--- Checking prerequisites ---"

if ! command -v go &>/dev/null; then
    echo "Error: Go is not installed."
    echo "  Install via: brew install go"
    exit 1
fi
echo "Go: $(go version)"

if ! command -v wget &>/dev/null && ! command -v curl &>/dev/null; then
    echo "Error: Neither wget nor curl found."
    exit 1
fi

# Use wget if available, otherwise fall back to curl
download() {
    local url="$1"
    local output="$2"
    if command -v wget &>/dev/null; then
        wget -q --show-progress "$url" -O "$output"
    else
        curl -fSL --progress-bar "$url" -o "$output"
    fi
}

# --- Create directories ---
mkdir -p "${MODELS_DIR}"/{kws,vad,asr,piper,voiceprint}

# --- Download sherpa-onnx models ---

# 1. Keyword spotting (wake word) model
echo ""
echo "--- Downloading wake word model ---"
if [ ! -f "${MODELS_DIR}/kws/tokens.txt" ]; then
    TMPFILE=$(mktemp /tmp/kws-model.XXXXXX.tar.bz2)
    download \
        "https://github.com/k2-fsa/sherpa-onnx/releases/download/kws-models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01.tar.bz2" \
        "$TMPFILE"
    tar xjf "$TMPFILE" -C /tmp
    cp /tmp/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/*.onnx "${MODELS_DIR}/kws/"
    cp /tmp/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/tokens.txt "${MODELS_DIR}/kws/"
    cp /tmp/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/keywords.txt "${MODELS_DIR}/kws/"
    # 追加自定义唤醒词"你好小派"
    if ! grep -q "@你好小派" "${MODELS_DIR}/kws/keywords.txt"; then
        echo 'n ǐ h ǎo x iǎo p ài @你好小派' >> "${MODELS_DIR}/kws/keywords.txt"
    fi
    rm -rf "$TMPFILE" /tmp/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01
    echo "Wake word model downloaded."
else
    echo "Wake word model already exists, skipping."
fi

# 2. Silero VAD model
echo ""
echo "--- Downloading VAD model ---"
if [ ! -f "${MODELS_DIR}/vad/silero_vad.onnx" ]; then
    download \
        "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/silero_vad.onnx" \
        "${MODELS_DIR}/vad/silero_vad.onnx"
    echo "VAD model downloaded."
else
    echo "VAD model already exists, skipping."
fi

# 3. Streaming ASR model (bilingual zh-en)
echo ""
echo "--- Downloading ASR model ---"
if [ ! -f "${MODELS_DIR}/asr/tokens.txt" ]; then
    TMPFILE=$(mktemp /tmp/asr-model.XXXXXX.tar.bz2)
    download \
        "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20.tar.bz2" \
        "$TMPFILE"
    tar xjf "$TMPFILE" -C /tmp
    cp /tmp/sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/*.onnx "${MODELS_DIR}/asr/"
    cp /tmp/sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/tokens.txt "${MODELS_DIR}/asr/"
    rm -rf "$TMPFILE" /tmp/sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20
    echo "ASR model downloaded."
else
    echo "ASR model already exists, skipping."
fi

# 4. Piper TTS model (optional — only needed if tts.engine is "piper")
echo ""
echo "--- Downloading Piper TTS model (optional) ---"
if [ ! -f "${MODELS_DIR}/piper/zh_CN-huayan-medium.onnx" ]; then
    download \
        "https://huggingface.co/rhasspy/piper-voices/resolve/main/zh/zh_CN/huayan/medium/zh_CN-huayan-medium.onnx" \
        "${MODELS_DIR}/piper/zh_CN-huayan-medium.onnx"
    download \
        "https://huggingface.co/rhasspy/piper-voices/resolve/main/zh/zh_CN/huayan/medium/zh_CN-huayan-medium.onnx.json" \
        "${MODELS_DIR}/piper/zh_CN-huayan-medium.onnx.json"
    echo "Piper TTS model downloaded."
else
    echo "Piper TTS model already exists, skipping."
fi

# 5. Speaker recognition model
echo ""
if [ ! -f "${MODELS_DIR}/voiceprint/3dspeaker_speech_campplus_sv_zh-cn_16k-common.onnx" ]; then
    download \
        "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/3dspeaker_speech_campplus_sv_zh-cn_16k-common.onnx" \
        "${MODELS_DIR}/voiceprint/3dspeaker_speech_campplus_sv_zh-cn_16k-common.onnx"
    echo "Speaker recognition model downloaded."
else
    echo "Speaker recognition model already exists, skipping."
fi

# --- NeteaseCloudMusicApi (optional, for music playback) ---
echo ""
echo "--- Setting up NeteaseCloudMusicApi (optional) ---"
NETEASE_DIR="${PIBUDDY_DIR}/NeteaseCloudMusicApi"
if command -v node &>/dev/null; then
    echo "Node.js: $(node --version)"
    if [ ! -d "${NETEASE_DIR}" ]; then
        echo "Cloning NeteaseCloudMusicApi..."
        git clone --depth 1 https://github.com/Binaryify/NeteaseCloudMusicApi.git "${NETEASE_DIR}"
    else
        echo "NeteaseCloudMusicApi already exists, updating..."
        cd "${NETEASE_DIR}"
        git pull || true
    fi
    cd "${NETEASE_DIR}"
    npm install --silent
    echo "NeteaseCloudMusicApi installed at ${NETEASE_DIR}"
else
    echo "Node.js not found, skipping NeteaseCloudMusicApi."
    echo "  Install Node.js via: brew install node"
fi

# --- Build ---
echo ""
echo "--- Building PiBuddy ---"
cd "${PIBUDDY_DIR}"
CGO_ENABLED=1 go build -o bin/pibuddy ./cmd/pibuddy
echo "Built bin/pibuddy"

# --- Audio check ---
echo ""
echo "--- Audio check ---"
# macOS: use system_profiler to list audio devices
if command -v system_profiler &>/dev/null; then
    echo "Audio devices:"
    system_profiler SPAudioDataType 2>/dev/null | grep -E "^\s+(Default|Device Name|Manufacturer)" || true
    echo ""
    # Check microphone permission hint
    echo "Note: macOS may prompt for microphone permission on first run."
    echo "      Go to System Settings > Privacy & Security > Microphone to verify."
fi

echo ""
echo "=== Setup complete ==="
echo ""
echo "Next steps:"
echo "  1. Set your LLM API key:"
echo "       export PIBUDDY_LLM_API_KEY='your-key'"
echo ""
echo "  2. Update configs/pibuddy.yaml for DeepSeek (or other LLM):"
echo "       llm:"
echo "         api_url: \"https://api.deepseek.com/v1\""
echo "         model: \"deepseek-chat\""
echo ""
echo "  3. Start NeteaseCloudMusicApi (for music playback):"
echo "       cd ${NETEASE_DIR} && node app.js &"
echo ""
echo "  4. Run PiBuddy:"
echo "       ./bin/pibuddy -config configs/pibuddy.yaml"
echo ""
echo "  5. Say \"你好小派\" to wake up!"
