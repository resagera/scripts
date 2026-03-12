#!/usr/bin/env bash

set -Eeuo pipefail
shopt -s nullglob

export LC_ALL=C

BASE_DIR="/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo"
FONT="/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"

START_DATE="${1:-$(date -d 'yesterday' +%F)}"
FPS="${2:-10}"
START_TIME="${3:-06:00:00}"

# 1 = оставлять cache даже при успехе
# 0 = удалять cache при успехе
KEEP_CACHE_ON_SUCCESS=0

NEXT_DATE="$(date -d "${START_DATE} +1 day" +%F)"

START_DIR="${BASE_DIR}/${START_DATE}"
NEXT_DIR="${BASE_DIR}/${NEXT_DATE}"

CACHE_DIR="./cache"
HIST_DIR="./historic"

mkdir -p "${CACHE_DIR}" "${HIST_DIR}"

CACHE_DIR="$(cd "${CACHE_DIR}" && pwd -P)"
HIST_DIR="$(cd "${HIST_DIR}" && pwd -P)"
BASE_DIR="$(cd "${BASE_DIR}" && pwd -P)"

START_DIR="${BASE_DIR}/${START_DATE}"
NEXT_DIR="${BASE_DIR}/${NEXT_DATE}"

RUN_TS="$(date '+%Y%m%d_%H%M%S')"

LOG_FILE="${CACHE_DIR}/make_timeline_${START_DATE}_${RUN_TS}.log"
LIST_FILE="${CACHE_DIR}/list_${START_DATE}_${RUN_TS}.txt"
INPUT_FILE="${CACHE_DIR}/input_${START_DATE}_${RUN_TS}.txt"
TMP_IMG_DIR="${CACHE_DIR}/frames_${START_DATE}_${RUN_TS}"
OUTPUT_FILE="${HIST_DIR}/timeline_${NEXT_DATE}_${FPS}fps_from_${START_TIME//:/-}.mp4"

mkdir -p "${TMP_IMG_DIR}"

exec > >(tee -a "${LOG_FILE}") 2>&1

SUCCESS=0

human_size() {
  local bytes="${1:-0}"
  awk -v b="$bytes" '
    function human(x) {
      s="B KB MB GB TB PB"
      split(s, arr, " ")
      i=1
      while (x >= 1024 && i < length(arr)) {
        x /= 1024
        i++
      }
      return sprintf("%.2f %s", x, arr[i])
    }
    BEGIN { print human(b) }
  '
}

to_hhmmss() {
  local t="$1"
  t="${t//:/}"
  printf "%06d" "$t"
}

strip_all_extensions() {
  local name="$1"
  while [[ "$name" == *.* ]]; do
    name="${name%.*}"
  done
  printf '%s' "$name"
}

escape_drawtext() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//:/\\:}"
  s="${s//\'/\\\'}"
  s="${s//%/\\%}"
  s="${s//,/\\,}"
  s="${s//[/\\[}"
  s="${s//]/\\]}"
  printf '%s' "$s"
}

cleanup() {
  local exit_code=$?

  if [[ "$SUCCESS" -eq 1 && "$KEEP_CACHE_ON_SUCCESS" -eq 0 ]]; then
    echo "Cleanup temporary files..."
    rm -f "${LIST_FILE}" "${INPUT_FILE}" 2>/dev/null || true
    rm -rf "${TMP_IMG_DIR}" 2>/dev/null || true
  else
    echo "Temporary files kept:"
    echo "  LIST_FILE  = ${LIST_FILE}"
    echo "  INPUT_FILE = ${INPUT_FILE}"
    echo "  TMP_IMG_DIR= ${TMP_IMG_DIR}"
  fi

  exit "$exit_code"
}
trap cleanup EXIT

echo "Start $(date '+%Y-%m-%d %H:%M:%S')"
echo "Log file            : ${LOG_FILE}"
echo "Start date          : ${START_DATE}"
echo "Next date           : ${NEXT_DATE}"
echo "FPS                 : ${FPS}"
echo "Start time          : ${START_TIME}"
echo "Base dir            : ${BASE_DIR}"
echo "Start dir           : ${START_DIR}"
echo "Next dir            : ${NEXT_DIR}"
echo "Temporary img dir   : ${TMP_IMG_DIR}"
echo "Output file         : ${OUTPUT_FILE}"

[[ -d "${START_DIR}" ]] || { echo "Ошибка: папка не найдена: ${START_DIR}" >&2; exit 1; }
[[ -d "${NEXT_DIR}" ]] || { echo "Ошибка: папка следующего дня не найдена: ${NEXT_DIR}" >&2; exit 1; }
[[ -f "${FONT}" ]] || { echo "Ошибка: шрифт не найден: ${FONT}" >&2; exit 1; }

START_HHMMSS="$(to_hhmmss "${START_TIME}")"

[[ "${START_HHMMSS}" =~ ^[0-9]{6}$ ]] || {
  echo "Ошибка: время должно быть в формате HH:MM:SS" >&2
  exit 1
}

[[ "${FPS}" =~ ^[0-9]+$ ]] || {
  echo "Ошибка: FPS должен быть целым положительным числом" >&2
  exit 1
}

[[ "${FPS}" -gt 0 ]] || {
  echo "Ошибка: FPS должен быть больше 0" >&2
  exit 1
}

FRAME_DURATION="$(awk -v fps="$FPS" 'BEGIN { printf "%.10f", 1/fps }')"

: > "${LIST_FILE}"
: > "${INPUT_FILE}"

selected_count=0
selected_bytes=0
last_out_file=""

declare -a SELECTED_FILES=()

add_files_from_dir() {
  local dir="$1"
  local mode="$2"
  local path base hhmmss size

  for path in "${dir}"/*.jpg; do
    [[ -f "$path" ]] || continue

    base="$(basename "$path")"
    hhmmss="$(printf '%s\n' "$base" | sed -n 's/.*_\([0-9]\{6\}\)\..*/\1/p')"
    [[ -n "$hhmmss" ]] || continue

    case "$mode" in
      ge_start)
        [[ "$hhmmss" < "$START_HHMMSS" ]] && continue
        ;;
      lt_start)
        [[ "$hhmmss" > "$START_HHMMSS" || "$hhmmss" == "$START_HHMMSS" ]] && continue
        ;;
      *)
        echo "Ошибка: неизвестный режим ${mode}" >&2
        exit 1
        ;;
    esac

    SELECTED_FILES+=("$path")

    size="$(stat -c '%s' "$path")"
    selected_bytes=$((selected_bytes + size))
    selected_count=$((selected_count + 1))
  done
}

add_files_from_dir "${START_DIR}" "ge_start"
add_files_from_dir "${NEXT_DIR}" "lt_start"

[[ "${#SELECTED_FILES[@]}" -gt 0 ]] || {
  echo "Ошибка: не найдено файлов для обработки" >&2
  exit 1
}

printf '%s\n' "${SELECTED_FILES[@]}" > "${LIST_FILE}"

echo "Selected files count: ${selected_count}"
echo "Selected files size : $(human_size "${selected_bytes}")"
echo "Rendering text onto temporary images..."

idx=0
for file in "${SELECTED_FILES[@]}"; do
  idx=$((idx + 1))

  [[ -f "$file" ]] || {
    echo "Ошибка: исходный файл не существует: $file" >&2
    exit 1
  }

  base="$(basename "$file")"
  label="$(strip_all_extensions "$base")"
  label_escaped="$(escape_drawtext "$label")"

  out_file="${TMP_IMG_DIR}/$(printf '%06d' "$idx")_${base}"

#echo "[$idx/$selected_count]"
#  echo "  input : $file"
#  echo "  output: $out_file"

  ffmpeg -y -loglevel error \
    -i "$file" \
    -vf "drawtext=fontfile=${FONT}:text='${label_escaped}':fontsize=24:fontcolor=white:x=(w-text_w)/2:y=h-th-10:box=1:boxcolor=black@0.5:boxborderw=8" \
    -q:v 1 \
    "$out_file" || {
      echo "Ошибка обработки $file" >&2
      exit 1
    }

  [[ -f "$out_file" ]] || {
    echo "Ошибка: временный файл не создан: $out_file" >&2
    exit 1
  }

  printf "file '%s'\n" "$out_file" >> "${INPUT_FILE}"
  printf "duration %s\n" "$FRAME_DURATION" >> "${INPUT_FILE}"

  last_out_file="$out_file"
done

if [[ -n "$last_out_file" ]]; then
  printf "file '%s'\n" "$last_out_file" >> "${INPUT_FILE}"
fi

tmp_frames_bytes="$(find "${TMP_IMG_DIR}" -type f -name '*.jpg' -printf '%s\n' | awk '{s+=$1} END {print s+0}')"

echo "Temporary rendered frames size: $(human_size "${tmp_frames_bytes}")"
echo "Building video..."

ffmpeg -y \
  -f concat -safe 0 -i "${INPUT_FILE}" \
  -vf "format=yuv420p" \
  "${OUTPUT_FILE}" || {
    echo "Ошибка сборки итогового видео" >&2
    exit 1
  }

[[ -f "${OUTPUT_FILE}" ]] || {
  echo "Ошибка: итоговое видео не создано" >&2
  exit 1
}

video_bytes="$(stat -c '%s' "${OUTPUT_FILE}")"

echo "Final video file    : ${OUTPUT_FILE}"
echo "Final video size    : $(human_size "${video_bytes}")"
echo "End $(date '+%Y-%m-%d %H:%M:%S')"

SUCCESS=1