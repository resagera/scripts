внутри /home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/ у меня есть папки вида 2026-03-09 и 2026-03-10 и подобные, в каждой папке файлы вида front_cam20260309_000003.jpg.jpg, front_cam20260309_000009.jpg.jpg, front_cam20260309_000058.jpg.jpg
Напиши скрипт, который берет файлы с 6 утра из одной папки и заканчивает файлами 6 утра второго файла.
Ориентируйся на команду, которая умеет только по одной папке делать обработку:
echo "Start $(date '+%Y-%m-%d %H:%M:%S')" && cd '/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2025-12-27' && ls *.jpg | sort > list.txt && sed 's/^/file /' list.txt > input.txt && ffmpeg -f concat -safe 0 -i input.txt -vf "fps=10,format=yuv420p" ../timeline_2025-12-28_10fps.mp4 && echo "End $(date '+%Y-%m-%d %H:%M:%S')"

скрипт должен принимать параметры:
1) дата папки, с которой начинается (по умолчанию вчера)
2) сколько fps поставить в команде ffmpeg (по умолчанию 10)

скрипт должен создавать папку ./cache, в которой временные файлы создает и видеофайл сохранять в папку ./historic
скрипт должен подсчитать сколько занимают места (в мегабайтах или гигабайтах) выбранные файлы и затем сколько места занимает видеофайл

----------
Вот готовый `bash`-скрипт.

Он делает так:

* берет папку стартовой даты, по умолчанию `вчера`
* берет файлы **с 06:00:00 стартового дня**
* потом добавляет файлы из **следующей папки до 05:59:59**
* собирает `mp4` через `ffmpeg`
* создает `./cache` для временных файлов
* сохраняет итоговое видео в `./historic`
* считает размер выбранных jpg и размер итогового видео

Скрипт:

```bash
#!/usr/bin/env bash

set -Eeuo pipefail

BASE_DIR="/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo"

START_DATE="${1:-$(date -d 'yesterday' +%F)}"
FPS="${2:-10}"

NEXT_DATE="$(date -d "${START_DATE} +1 day" +%F)"

START_DIR="${BASE_DIR}/${START_DATE}"
NEXT_DIR="${BASE_DIR}/${NEXT_DATE}"

CACHE_DIR="./cache"
HIST_DIR="./historic"

LIST_FILE="${CACHE_DIR}/list_${START_DATE}.txt"
INPUT_FILE="${CACHE_DIR}/input_${START_DATE}.txt"
OUTPUT_FILE="${HIST_DIR}/timeline_${NEXT_DATE}_${FPS}fps.mp4"

mkdir -p "${CACHE_DIR}" "${HIST_DIR}"

if [[ ! -d "${START_DIR}" ]]; then
  echo "Ошибка: папка не найдена: ${START_DIR}" >&2
  exit 1
fi

if [[ ! -d "${NEXT_DIR}" ]]; then
  echo "Ошибка: папка следующего дня не найдена: ${NEXT_DIR}" >&2
  exit 1
fi

tmp_selected="${CACHE_DIR}/selected_${START_DATE}.tmp"
: > "${tmp_selected}"
: > "${LIST_FILE}"
: > "${INPUT_FILE}"

selected_count=0
selected_bytes=0

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

add_files_from_dir() {
  local dir="$1"
  local mode="$2"   # ge060000 | lt060000
  local f base hhmmss full size

  while IFS= read -r -d '' f; do
    base="$(basename "$f")"

    # Ищем время по шаблону *_HHMMSS.*
    if [[ "$base" =~ _([0-9]{6})\. ]]; then
      hhmmss="${BASH_REMATCH[1]}"
    else
      continue
    fi

    case "$mode" in
      ge060000)
        [[ "$hhmmss" < "060000" ]] && continue
        ;;
      lt060000)
        [[ "$hhmmss" > "055959" ]] && continue
        ;;
      *)
        echo "Внутренняя ошибка: неизвестный режим $mode" >&2
        exit 1
        ;;
    esac

    full="$f"
    printf '%s\n' "$full" >> "${tmp_selected}"

    size="$(stat -c '%s' "$full")"
    selected_bytes=$((selected_bytes + size))
    selected_count=$((selected_count + 1))
  done < <(find "$dir" -maxdepth 1 -type f -name '*.jpg' -print0 | sort -z)
}

echo "Start $(date '+%Y-%m-%d %H:%M:%S')"
echo "Start date : ${START_DATE}"
echo "Next date  : ${NEXT_DATE}"
echo "FPS        : ${FPS}"
echo "Start dir  : ${START_DIR}"
echo "Next dir   : ${NEXT_DIR}"

add_files_from_dir "${START_DIR}" "ge060000"
add_files_from_dir "${NEXT_DIR}" "lt060000"

if [[ ! -s "${tmp_selected}" ]]; then
  echo "Ошибка: не найдено файлов для обработки" >&2
  exit 1
fi

sort "${tmp_selected}" > "${LIST_FILE}"

while IFS= read -r file; do
  # Для ffmpeg concat
  esc_file="${file//\'/\'\\\'\'}"
  printf "file '%s'\n" "$esc_file" >> "${INPUT_FILE}"
done < "${LIST_FILE}"

echo "Selected files count: ${selected_count}"
echo "Selected files size : $(human_size "${selected_bytes}")"

ffmpeg -y \
  -f concat \
  -safe 0 \
  -i "${INPUT_FILE}" \
  -vf "fps=${FPS},format=yuv420p" \
  "${OUTPUT_FILE}"

video_bytes="$(stat -c '%s' "${OUTPUT_FILE}")"

echo "Video file          : ${OUTPUT_FILE}"
echo "Video size          : $(human_size "${video_bytes}")"
echo "End $(date '+%Y-%m-%d %H:%M:%S')"
```

Как использовать:

```bash
chmod +x make_timeline.sh
./make_timeline.sh
```

По умолчанию:

* дата = вчера
* fps = 10

Примеры:

```bash
./make_timeline.sh 2026-03-09
./make_timeline.sh 2026-03-09 15
```

Что получится:

* временные файлы: `./cache/...`
* итоговое видео: `./historic/timeline_2026-03-10_10fps.mp4`

Если хочешь, я могу сразу дать улучшенную версию, которая еще:

* пишет лог в файл,
* показывает прогресс по количеству кадров,
* умеет выбирать не только 06:00, а любое время старта параметром.

----------

да, давай лог, возможность выбирать время + добавить чтобы в результирующем видео в кадрах было указано имя файла (без расширения, если возможно)

----------

Вот расширенная версия.

Что добавил:

* лог в файл
* выбор времени старта отдельным параметром
* в кадре рисуется имя файла без расширения
* временные файлы в `./cache`
* итоговое видео в `./historic`

Параметры:

1. дата стартовой папки, по умолчанию `вчера`
2. fps, по умолчанию `10`
3. время старта, по умолчанию `06:00:00`

Примеры:

```bash
./make_timeline.sh
./make_timeline.sh 2026-03-09
./make_timeline.sh 2026-03-09 12
./make_timeline.sh 2026-03-09 12 05:30:00
```

Скрипт:

```bash
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
LOG_DIR="./cache"

mkdir -p "${CACHE_DIR}" "${HIST_DIR}" "${LOG_DIR}"

RUN_TS="$(date '+%Y%m%d_%H%M%S')"
LOG_FILE="${LOG_DIR}/make_timeline_${START_DATE}_${RUN_TS}.log"

LIST_FILE="${CACHE_DIR}/list_${START_DATE}_${RUN_TS}.txt"
INPUT_FILE="${CACHE_DIR}/input_${START_DATE}_${RUN_TS}.txt"
FILTER_FILE="${CACHE_DIR}/filter_${START_DATE}_${RUN_TS}.txt"
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

escape_drawtext() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//:/\\:}"
  s="${s//\'/\\\'}"
  s="${s//[/\\[}"
  s="${s//]/\\]}"
  s="${s//,/\\,}"
  s="${s//%/\\%}"
  printf '%s' "$s"
}

escape_concat_path() {
  local s="$1"
  s="${s//\'/\'\\\'\'}"
  printf '%s' "$s"
}

cleanup() {
  echo "Cleanup temporary files..."
}
trap cleanup EXIT

echo "Start $(date '+%Y-%m-%d %H:%M:%S')"
echo "Log file            : ${LOG_FILE}"
echo "Start date          : ${START_DATE}"
echo "Next date           : ${NEXT_DATE}"
echo "FPS                 : ${FPS}"
echo "Start time          : ${START_TIME}"
echo "Start dir           : ${START_DIR}"
echo "Next dir            : ${NEXT_DIR}"
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

if [[ ! "${START_HHMMSS}" =~ ^[0-9]{6}$ ]]; then
  echo "Ошибка: время должно быть в формате HH:MM:SS" >&2
  exit 1
fi

: > "${TMP_SELECTED}"
: > "${LIST_FILE}"
: > "${INPUT_FILE}"
: > "${FILTER_FILE}"

selected_count=0
selected_bytes=0

add_files_from_dir() {
  local dir="$1"
  local mode="$2" # ge_start | lt_start
  local f base hhmmss size

  while IFS= read -r -d '' f; do
    base="$(basename "$f")"

    if [[ "$base" =~ _([0-9]{6})\. ]]; then
      hhmmss="${BASH_REMATCH[1]}"
    else
      continue
    fi

    case "$mode" in
      ge_start)
        [[ "$hhmmss" < "${START_HHMMSS}" ]] && continue
        ;;
      lt_start)
        [[ "$hhmmss" >= "${START_HHMMSS}" ]] && continue
        ;;
      *)
        echo "Ошибка: неизвестный режим ${mode}" >&2
        exit 1
        ;;
    esac

    printf '%s\n' "$f" >> "${TMP_SELECTED}"

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

echo "Selected files count: ${selected_count}"
echo "Selected files size : $(human_size "${selected_bytes}")"

idx=0
while IFS= read -r file; do
  idx=$((idx + 1))
  base="$(basename "$file")"
  label="$(strip_all_extensions "$base")"
  label_escaped="$(escape_drawtext "$label")"
  file_escaped="$(escape_concat_path "$file")"

  printf "file '%s'\n" "$file_escaped" >> "${INPUT_FILE}"

  cat >> "${FILTER_FILE}" <<EOF
[$idx:v]drawtext=text='${label_escaped}':x=20:y=h-th-20:fontcolor=white:fontsize=24:box=1:boxcolor=black@0.55:boxborderw=8[v${idx}];
EOF
done < "${LIST_FILE}"

{
  for i in $(seq 1 "${selected_count}"); do
    printf "[%d:v]" "$i"
  done
  printf "concat=n=%d:v=1:a=0,format=yuv420p,fps=%s[vout]\n" "${selected_count}" "${FPS}"
} >> "${FILTER_FILE}"

echo "Preparing annotated video with filenames in frame..."

ffmpeg -y \
  $(while IFS= read -r f; do printf -- "-loop 1 -i %q " "$f"; done < "${LIST_FILE}") \
  -filter_complex "
$(cat "${FILTER_FILE}")
" \
  -map "[vout]" \
  -vsync vfr \
  "${OUTPUT_FILE}"

video_bytes="$(stat -c '%s' "${OUTPUT_FILE}")"

echo "Video file          : ${OUTPUT_FILE}"
echo "Video size          : $(human_size "${video_bytes}")"
echo "End $(date '+%Y-%m-%d %H:%M:%S')"
```

Важный момент: в этой версии имя файла рисуется на кадре через `drawtext`, но для этого у `ffmpeg` должна быть поддержка `drawtext` и обычно `libfreetype`.

Проверить можно так:

```bash
ffmpeg -filters | grep drawtext
```

Если `drawtext` есть, всё должно работать.

Но у этой реализации есть одна тонкость: она подает каждый файл как отдельный вход `-i`, и если кадров очень много, команда может стать слишком длинной. Для нескольких сотен или даже пары тысяч файлов это еще может жить, но для очень больших наборов уже неудобно.

Поэтому практичнее сделать более устойчивый вариант:

* сначала собрать обычное видео через concat
* отдельно подготовить файл с таймкодами и подписями
* потом вторым проходом наложить подписи

Либо еще лучше — через `ffmpeg` `-vf drawtext=textfile=...` или через промежуточный `ass`-субтитр файл. Это надежнее на больших объемах.

Ниже даю более надежную версию, которую я бы советовал использовать на практике. Она делает:

* сборку из списка
* создание `.srt` с подписью имени файла на каждом кадре
* наложение субтитров на итоговое видео

Это устойчивее для больших списков.

```bash
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

    if [[ "$base" =~ _([0-9]{6})\. ]]; then
      hhmmss="${BASH_REMATCH[1]}"
    else
      continue
    fi

    case "$mode" in
      ge_start)
        [[ "$hhmmss" < "${START_HHMMSS}" ]] && continue
        ;;
      lt_start)
        [[ "$hhmmss" >= "${START_HHMMSS}" ]] && continue
        ;;
      *)
        echo "Ошибка: неизвестный режим ${mode}" >&2
        exit 1
        ;;
    esac

    printf '%s\n' "$f" >> "${TMP_SELECTED}"
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
```

Я бы советовал именно вторую версию с `srt`, потому что она:

* стабильнее
* проще отлаживается
* лучше работает на большом количестве кадров

И еще важное замечание: в твоей исходной команде `ffmpeg -f concat` без `-framerate` и с `-vf "fps=10"` поведение может быть не совсем тем, которое ожидается для набора изображений. Если тебе нужно именно “каждый jpg = один кадр таймлапса”, лучше при желании можно переделать логику на `-framerate` или на image2-паттерн, но так как у тебя файлы из двух папок и список произвольный, текущий подход с concat + subtitles нормальный.

Я могу сразу сделать еще одну версию, где:

* поверх имени файла будет еще и дата/время из имени,
* будет прогресс-бар через `ffmpeg -progress`,
* и можно будет передать еще четвертый параметр: конечное время вместо “до следующего дня того же времени”.


----------
./gen_timeline.sh: строка 130: синтаксическая ошибка в условном выражении

----------


