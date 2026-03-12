#!/usr/bin/env bash

set -Eeuo pipefail

BASE_DIR="/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo"

START_DATE="${1:-$(date -d 'yesterday' +%F)}"
FPS="${2:-10}"
START_TIME="${3:-06:00:00}"

NEXT_DATE="$(date -d "${START_DATE} +1 day" +%F)"

START_DIR="${BASE_DIR}/${START_DATE}"
NEXT_DIR="${BASE_DIR}/${NEXT_DATE}"

CACHE_DIR="./cache"
HIST_DIR="./historic"
mkdir -p "${CACHE_DIR}" "${HIST_DIR}"

RUN_TS="$(date '+%Y%m%d_%H%M%S')"
LOG_FILE="${CACHE_DIR}/make_timeline_${START_DATE}_${RUN_TS}.log"

LIST_FILE="${CACHE_DIR}/list_${START_DATE}_${RUN_TS}.txt"
INPUT_FILE="${CACHE_DIR}/input_${START_DATE}_${RUN_TS}.txt"
SRT_FILE="${CACHE_DIR}/labels_${START_DATE}_${RUN_TS}.srt"
RAW_VIDEO="${CACHE_DIR}/timeline_raw_${START_DATE}_${RUN_TS}.mp4"
OUTPUT_FILE="${HIST_DIR}/timeline_${NEXT_DATE}_${FPS}fps_from_${START_TIME//:/-}.mp4"
TMP_SELECTED="${CACHE_DIR}/selected_${START_DATE}_${RUN_TS}.tmp"

exec > >(tee -a "${LOG_FILE}") 2>&1

human_size() {
  local bytes="$1"
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
  printf "%06d" "${t}"
}

strip_all_extensions() {
  local name="$1"
  while [[ "$name" == *.* ]]; do
    name="${name%.*}"
  done
  printf '%s' "$name"
}

srt_time_from_index() {
  local idx="$1"
  local fps="$2"
  awk -v i="$idx" -v fps="$fps" '
    BEGIN {
      t = (i - 1) / fps
      h = int(t / 3600)
      t -= h * 3600
      m = int(t / 60)
      t -= m * 60
      s = int(t)
      ms = int((t - s) * 1000 + 0.5)
      if (ms == 1000) { ms = 0; s++ }
      if (s == 60) { s = 0; m++ }
      if (m == 60) { m = 0; h++ }
      printf "%02d:%02d:%02d,%03d", h, m, s, ms
    }
  '
}

echo "Start $(date '+%Y-%m-%d %H:%M:%S')"
echo "Log file            : ${LOG_FILE}"
echo "Start date          : ${START_DATE}"
echo "Next date           : ${NEXT_DATE}"
echo "FPS                 : ${FPS}"
echo "Start time          : ${START_TIME}"
echo "Output file         : ${OUTPUT_FILE}"

if [[ ! -d "${START_DIR}" ]]; then
  echo "Ошибка: папка не найдена: ${START_DIR}" >&2
  exit 1
fi

if [[ ! -d "${NEXT_DIR}" ]]; then
  echo "Ошибка: папка следующего дня не найдена: ${NEXT_DIR}" >&2
  exit 1
fi

START_HHMMSS="$(to_hhmmss "${START_TIME}")"

: > "${TMP_SELECTED}"
: > "${LIST_FILE}"
: > "${INPUT_FILE}"
: > "${SRT_FILE}"

selected_count=0
selected_bytes=0

add_files_from_dir() {
  local dir="$1"
  local mode="$2"
  local f base hhmmss size

  while IFS= read -r -d '' f; do
    base="$(basename "$f")"

    hhmmss="$(printf '%s\n' "$base" | sed -n 's/.*_\([0-9]\{6\}\)\..*/\1/p')"
    [[ -z "$hhmmss" ]] && continue

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

    printf '%s\n' "$f" >> "$TMP_SELECTED"

    size="$(stat -c '%s' "$f")"
    selected_bytes=$((selected_bytes + size))
    selected_count=$((selected_count + 1))
  done < <(find "$dir" -maxdepth 1 -type f -name '*.jpg' -print0 | sort -z)
}

add_files_from_dir "${START_DIR}" "ge_start"
add_files_from_dir "${NEXT_DIR}" "lt_start"

if [[ ! -s "${TMP_SELECTED}" ]]; then
  echo "Ошибка: не найдено файлов для обработки" >&2
  exit 1
fi

sort "${TMP_SELECTED}" > "${LIST_FILE}"

while IFS= read -r file; do
  printf "file '%s'\n" "${file//\'/\'\\\'\'}" >> "${INPUT_FILE}"
done < "${LIST_FILE}"

echo "Selected files count: ${selected_count}"
echo "Selected files size : $(human_size "${selected_bytes}")"

echo "Building raw video..."
ffmpeg -y \
  -f concat -safe 0 -i "${INPUT_FILE}" \
  -vf "fps=${FPS},format=yuv420p" \
  "${RAW_VIDEO}"

echo "Building subtitles with filenames..."
idx=1
while IFS= read -r file; do
  label="$(strip_all_extensions "$(basename "$file")")"
  start_ts="$(srt_time_from_index "$idx" "$FPS")"
  end_ts="$(srt_time_from_index "$((idx + 1))" "$FPS")"

  {
    echo "${idx}"
    echo "${start_ts} --> ${end_ts}"
    echo "${label}"
    echo
  } >> "${SRT_FILE}"

  idx=$((idx + 1))
done < "${LIST_FILE}"

echo "Burning subtitles..."
ffmpeg -y \
  -i "${RAW_VIDEO}" \
  -vf "subtitles='${SRT_FILE}':force_style='Fontsize=18,PrimaryColour=&HFFFFFF&,BackColour=&H80000000&,BorderStyle=4,MarginV=20'" \
  "${OUTPUT_FILE}"

raw_bytes="$(stat -c '%s' "${RAW_VIDEO}")"
video_bytes="$(stat -c '%s' "${OUTPUT_FILE}")"

echo "Raw video file      : ${RAW_VIDEO}"
echo "Raw video size      : $(human_size "${raw_bytes}")"
echo "Final video file    : ${OUTPUT_FILE}"
echo "Final video size    : $(human_size "${video_bytes}")"
echo "End $(date '+%Y-%m-%d %H:%M:%S')"