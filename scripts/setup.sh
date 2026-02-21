#!/usr/bin/env bash
set -euo pipefail

# PiBuddy — Raspberry Pi setup script
# Run this on the Raspberry Pi to install dependencies and download models.

PIBUDDY_DIR="${PIBUDDY_DIR:-/home/pi/pibuddy}"
MODELS_DIR="${PIBUDDY_DIR}/models"

echo "=== PiBuddy Setup ==="
echo "Install dir: ${PIBUDDY_DIR}"

# --- System dependencies ---
echo ""
echo "--- Installing system packages ---"
sudo apt-get update
sudo apt-get install -y \
    alsa-utils \
    libasound2-dev \
    wget \
    unzip

# --- Create directories ---
mkdir -p "${MODELS_DIR}"/{kws,vad,asr,piper,voiceprint}

# --- Download sherpa-onnx models ---

# 1. Keyword spotting (wake word) model
echo ""
echo "--- Downloading wake word model ---"
if [ ! -f "${MODELS_DIR}/kws/tokens.txt" ]; then
    cd /tmp
    wget -q --show-progress \
        "https://github.com/k2-fsa/sherpa-onnx/releases/download/kws-models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01.tar.bz2" \
        -O kws-model.tar.bz2
    tar xjf kws-model.tar.bz2
    cp sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/*.onnx "${MODELS_DIR}/kws/"
    cp sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/tokens.txt "${MODELS_DIR}/kws/"
    cp sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/keywords.txt "${MODELS_DIR}/kws/"
    # 追加自定义唤醒词"你好小派"
    if ! grep -q "@你好小派" "${MODELS_DIR}/kws/keywords.txt"; then
        echo 'n ǐ h ǎo x iǎo p ài @你好小派' >> "${MODELS_DIR}/kws/keywords.txt"
    fi
    rm -rf kws-model.tar.bz2 sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01
    echo "Wake word model downloaded."
else
    echo "Wake word model already exists, skipping."
fi

# 2. Silero VAD model
echo ""
echo "--- Downloading VAD model ---"
if [ ! -f "${MODELS_DIR}/vad/silero_vad.onnx" ]; then
    wget -q --show-progress \
        "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/silero_vad.onnx" \
        -O "${MODELS_DIR}/vad/silero_vad.onnx"
    echo "VAD model downloaded."
else
    echo "VAD model already exists, skipping."
fi

# 3. Streaming ASR model (bilingual zh-en)
echo ""
echo "--- Downloading ASR model ---"
if [ ! -f "${MODELS_DIR}/asr/tokens.txt" ]; then
    cd /tmp
    wget -q --show-progress \
        "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20.tar.bz2" \
        -O asr-model.tar.bz2
    tar xjf asr-model.tar.bz2
    cp sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/*.onnx "${MODELS_DIR}/asr/"
    cp sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/tokens.txt "${MODELS_DIR}/asr/"
    rm -rf asr-model.tar.bz2 sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20
    echo "ASR model downloaded."
else
    echo "ASR model already exists, skipping."
fi

# 4. Piper TTS model (optional fallback)
echo ""
echo "--- Downloading Piper TTS model (optional) ---"
if [ ! -f "${MODELS_DIR}/piper/zh_CN-huayan-medium.onnx" ]; then
    wget -q --show-progress \
        "https://huggingface.co/rhasspy/piper-voices/resolve/main/zh/zh_CN/huayan/medium/zh_CN-huayan-medium.onnx" \
        -O "${MODELS_DIR}/piper/zh_CN-huayan-medium.onnx"
    wget -q --show-progress \
        "https://huggingface.co/rhasspy/piper-voices/resolve/main/zh/zh_CN/huayan/medium/zh_CN-huayan-medium.onnx.json" \
        -O "${MODELS_DIR}/piper/zh_CN-huayan-medium.onnx.json"
    echo "Piper TTS model downloaded."
else
    echo "Piper TTS model already exists, skipping."
fi

# 5. Speaker recognition model
echo ""
if [ ! -f "${MODELS_DIR}/voiceprint/3dspeaker_speech_campplus_sv_zh-cn_16k-common.onnx" ]; then
    wget -q --show-progress \
        "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/3dspeaker_speech_campplus_sv_zh-cn_16k-common.onnx" \
        -O "${MODELS_DIR}/voiceprint/3dspeaker_speech_campplus_sv_zh-cn_16k-common.onnx"
    echo "Speaker recognition model downloaded."
else
    echo "Speaker recognition model already exists, skipping."
fi

# --- Audio test ---
echo ""
echo "--- Audio check ---"
echo "Available capture devices:"
arecord -l 2>/dev/null || echo "  (no capture devices found — check microphone connection)"
echo ""
echo "Available playback devices:"
aplay -l 2>/dev/null || echo "  (no playback devices found — check speaker connection)"

echo ""
echo "=== Setup complete ==="
echo ""
echo "Next steps:"
echo "  1. Set your API keys (add to ~/.bashrc or ~/.zshrc):"
echo ""
echo "     # LLM Provider Keys (multi-provider support)"
echo "     export PIBUDDY_QWEN_API_KEY='your-qwen-key'          # 通义千问 (推荐，免费400万token)"
echo "     export PIBUDDY_HUNYUAN_API_KEY='your-hunyuan-key'    # 腾讯混元 (免费100万token)"
echo "     export PIBUDDY_ARK_API_KEY='your-ark-key'            # 火山方舟 (免费50万token)"
echo "     export PIBUDDY_ARK_ENDPOINT_ID='your-endpoint-id'    # 火山方舟接入点ID"
echo "     export PIBUDDY_LLM_API_KEY='your-deepseek-key'       # DeepSeek (付费兜底)"
echo ""
echo "     # Tencent Cloud (TTS, ASR, Translate)"
echo "     export PIBUDDY_TENCENT_SECRET_ID='your-secret-id'"
echo "     export PIBUDDY_TENCENT_SECRET_KEY='your-secret-key'"
echo "     export PIBUDDY_TENCENT_APP_ID='your-app-id'"
echo ""
echo "     # Other Services"
echo "     export PIBUDDY_QWEATHER_API_KEY='your-qweather-key'  # 和风天气"
echo "     export PIBUDDY_HA_TOKEN='your-homeassistant-token'   # Home Assistant"
echo "     export PIBUDDY_EZVIZ_AK='your-ezviz-ak'              # 萤石门锁"
echo "     export PIBUDDY_EZVIZ_SK='your-ezviz-sk'"
echo ""
echo "  2. Edit config if needed:  nano ${PIBUDDY_DIR}/configs/pibuddy.yaml"
echo "  3. Run PiBuddy:            ${PIBUDDY_DIR}/pibuddy -config ${PIBUDDY_DIR}/configs/pibuddy.yaml"
echo "  4. Or enable the service:  sudo systemctl enable --now pibuddy"
