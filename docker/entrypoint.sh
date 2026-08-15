#!/usr/bin/env bash
set -euo pipefail

APP_ROOT="/opt/wire-pod"
DATA_ROOT="${WIREPOD_DATA_DIR:-/data}"
IMAGES_ROOT="${WIREPOD_IMAGES_DIR:-/images}"
DEFAULT_SOURCE="${APP_ROOT}/docker/default-source.sh"
RUN_AS_USER="wirepod"

# The image runs as an unprivileged user (see dockerfile), but a
# bind-mounted host directory (docker-compose's ./data, ./images) is
# created by the Docker daemon owned by root the first time -- wire-pod
# couldn't write to it otherwise. Self-heal here: if we're root, fix
# ownership only where it's actually wrong (skip the recursive chown on
# an already-correct, possibly large data dir on every restart), then
# drop to the unprivileged user for everything else, including the app
# itself. CAP_NET_BIND_SERVICE (set on the binary at build time) still
# lets that user bind :80/:443 afterward.
if [ "$(id -u)" = "0" ]; then
    mkdir -p "${DATA_ROOT}" "${IMAGES_ROOT}"
    target_uid="$(id -u "${RUN_AS_USER}")"
    for dir in "${DATA_ROOT}" "${IMAGES_ROOT}"; do
        if [ "$(stat -c %u "${dir}")" != "${target_uid}" ]; then
            chown -R "${RUN_AS_USER}:${RUN_AS_USER}" "${dir}"
        fi
    done
    exec setpriv --reuid="${RUN_AS_USER}" --regid="${RUN_AS_USER}" --clear-groups "$0" "$@"
fi

mkdir -p "${DATA_ROOT}" "${IMAGES_ROOT}"

link_dir() {
    local rel_path="$1"
    local src_path="${APP_ROOT}/${rel_path}"
    local dest_path="${DATA_ROOT}/${rel_path}"

    mkdir -p "$(dirname "${dest_path}")"

    if [ ! -d "${dest_path}" ]; then
        if [ -d "${src_path}" ]; then
            cp -a "${src_path}" "${dest_path}"
        else
            mkdir -p "${dest_path}"
        fi
    fi

    if [ -e "${src_path}" ] && [ ! -L "${src_path}" ]; then
        rm -rf "${src_path}"
    fi

    ln -sfn "${dest_path}" "${src_path}"
}

link_file() {
    local rel_path="$1"
    local src_path="${APP_ROOT}/${rel_path}"
    local dest_path="${DATA_ROOT}/${rel_path}"

    mkdir -p "$(dirname "${dest_path}")"

    if [ ! -e "${dest_path}" ]; then
        if [ -f "${src_path}" ]; then
            cp -a "${src_path}" "${dest_path}"
        else
            : >"${dest_path}"
        fi
    fi

    if [ -e "${src_path}" ] && [ ! -L "${src_path}" ]; then
        rm -f "${src_path}"
    fi

    ln -sfn "${dest_path}" "${src_path}"
}

link_file_with_default() {
    local rel_path="$1"
    local default_path="$2"
    local src_path="${APP_ROOT}/${rel_path}"
    local dest_path="${DATA_ROOT}/${rel_path}"

    mkdir -p "$(dirname "${dest_path}")"

    if [ ! -e "${dest_path}" ]; then
        if [ -n "${default_path}" ] && [ -f "${default_path}" ]; then
            cp -a "${default_path}" "${dest_path}"
        elif [ -f "${src_path}" ]; then
            cp -a "${src_path}" "${dest_path}"
        else
            : >"${dest_path}"
        fi
    fi

    if [ -e "${src_path}" ] && [ ! -L "${src_path}" ]; then
        rm -f "${src_path}"
    fi

    ln -sfn "${dest_path}" "${src_path}"
}

persist_directories() {
    link_dir certs
    link_dir stt
    link_dir vosk
    link_dir whisper.cpp
    link_dir vector-cloud/build
    link_dir chipper/jdocs
    link_dir chipper/plugins
    link_dir chipper/session-certs
}

persist_files() {
    link_file chipper/apiConfig.json
    link_file chipper/botConfig.json
    link_file chipper/customIntents.json
    link_file chipper/pico.key
    link_file chipper/useepod
    link_file_with_default chipper/source.sh "${DEFAULT_SOURCE}"
}

update_export() {
    local key="$1"
    local value="$2"
    local file_path="$3"

    local escaped
    escaped=$(printf '%s' "${value}" | sed 's/[\\&/]/\\&/g')

    if grep -q "^export ${key}=" "${file_path}"; then
        sed -i "s/^export ${key}=.*/export ${key}=\"${escaped}\"/" "${file_path}"
    else
        printf 'export %s="%s"\n' "${key}" "${value}" >>"${file_path}"
    fi
}

apply_env_overrides() {
    local source_file="${APP_ROOT}/chipper/source.sh"

    if [ -n "${WIREPOD_DEBUG_LOGGING:-}" ]; then
        update_export "DEBUG_LOGGING" "${WIREPOD_DEBUG_LOGGING}" "${source_file}"
    fi

    if [ -n "${WIREPOD_STT_SERVICE:-}" ]; then
        update_export "STT_SERVICE" "${WIREPOD_STT_SERVICE}" "${source_file}"
    fi

    if [ -n "${WIREPOD_STT_LANGUAGE:-}" ]; then
        update_export "STT_LANGUAGE" "${WIREPOD_STT_LANGUAGE}" "${source_file}"
    fi

    if [ -n "${WIREPOD_USE_INBUILT_BLE:-}" ]; then
        update_export "USE_INBUILT_BLE" "${WIREPOD_USE_INBUILT_BLE}" "${source_file}"
    fi

    if [ -n "${WIREPOD_PICOVOICE_APIKEY:-}" ]; then
        update_export "PICOVOICE_APIKEY" "${WIREPOD_PICOVOICE_APIKEY}" "${source_file}"
        printf '%s\n' "${WIREPOD_PICOVOICE_APIKEY}" >"${DATA_ROOT}/chipper/pico.key"
    fi

    if [ -n "${WIREPOD_STT_WHISPER_URL:-}" ]; then
        update_export "STT_WHISPER_URL" "${WIREPOD_STT_WHISPER_URL}" "${source_file}"
    fi

    if [ -n "${WIREPOD_STT_WHISPER_KEY:-}" ]; then
        update_export "STT_WHISPER_KEY" "${WIREPOD_STT_WHISPER_KEY}" "${source_file}"
    fi

    if [ -n "${WIREPOD_STT_WHISPER_MODEL:-}" ]; then
        update_export "STT_WHISPER_MODEL" "${WIREPOD_STT_WHISPER_MODEL}" "${source_file}"
    fi

    # Knowledge ("Ask") and weather settings, like everything above, only
    # ever seed the very first config (see vars.CreateConfigFromEnv) --
    # once apiConfig.json exists, the dashboard is authoritative and these
    # are ignored on subsequent restarts.
    if [ -n "${WIREPOD_KNOWLEDGE_ENABLED:-}" ]; then
        update_export "KNOWLEDGE_ENABLED" "${WIREPOD_KNOWLEDGE_ENABLED}" "${source_file}"
    fi

    if [ -n "${WIREPOD_KNOWLEDGE_PROVIDER:-}" ]; then
        update_export "KNOWLEDGE_PROVIDER" "${WIREPOD_KNOWLEDGE_PROVIDER}" "${source_file}"
    fi

    if [ -n "${WIREPOD_KNOWLEDGE_KEY:-}" ]; then
        update_export "KNOWLEDGE_KEY" "${WIREPOD_KNOWLEDGE_KEY}" "${source_file}"
    fi

    if [ -n "${WIREPOD_KNOWLEDGE_ID:-}" ]; then
        update_export "KNOWLEDGE_ID" "${WIREPOD_KNOWLEDGE_ID}" "${source_file}"
    fi

    if [ -n "${WIREPOD_WEATHERAPI_ENABLED:-}" ]; then
        update_export "WEATHERAPI_ENABLED" "${WIREPOD_WEATHERAPI_ENABLED}" "${source_file}"
    fi

    if [ -n "${WIREPOD_WEATHERAPI_PROVIDER:-}" ]; then
        update_export "WEATHERAPI_PROVIDER" "${WIREPOD_WEATHERAPI_PROVIDER}" "${source_file}"
    fi

    if [ -n "${WIREPOD_WEATHERAPI_KEY:-}" ]; then
        update_export "WEATHERAPI_KEY" "${WIREPOD_WEATHERAPI_KEY}" "${source_file}"
    fi

    if [ -n "${WIREPOD_WEATHERAPI_UNIT:-}" ]; then
        update_export "WEATHERAPI_UNIT" "${WIREPOD_WEATHERAPI_UNIT}" "${source_file}"
    fi
}

persist_directories
persist_files

if [ ! -e "${HOME}/.vosk" ]; then
    ln -sfn /opt/vosk "${HOME}/.vosk"
fi

apply_env_overrides

exec "$@"
