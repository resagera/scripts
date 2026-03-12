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


Скорее всего, ошибка в этом месте:

```bash
if [[ "$base" =~ _([0-9]{6})\. ]]; then
```

На некоторых системах `bash` ругается на такое регулярное выражение внутри `[[ ... =~ ... ]]`, особенно если там неудачно экранирована точка или скрипт случайно запускается не через `bash`.

Сделай надежнее: вообще уйдем от regex в `[[ =~ ]]` и просто вытащим время через `sed`. Это стабильнее.

Замени функцию `add_files_from_dir()` на такую:

```bash
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
```

И еще проверь, как запускаешь скрипт. Нужно именно так:

```bash
bash ./gen_timeline.sh
```

или

```bash
chmod +x ./gen_timeline.sh
./gen_timeline.sh
```

Потому что если запускать через `sh ./gen_timeline.sh`, то `[[ ... ]]` и process substitution:

```bash
done < <(...)
```

тоже могут ломаться.

Если хочешь, вот более безопасный вариант всего скрипта целиком, уже с этой правкой.

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
  printf "%06d" "$t"
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

[[ -d "${START_DIR}" ]] || { echo "Ошибка: папка не найдена: ${START_DIR}" >&2; exit 1; }
[[ -d "${NEXT_DIR}" ]] || { echo "Ошибка: папка следующего дня не найдена: ${NEXT_DIR}" >&2; exit 1; }

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

    printf '%s\n' "$f" >> "${TMP_SELECTED}"
    size="$(stat -c '%s' "$f")"
    selected_bytes=$((selected_bytes + size))
    selected_count=$((selected_count + 1))
  done < <(find "$dir" -maxdepth 1 -type f -name '*.jpg' -print0 | sort -z)
}

add_files_from_dir "${START_DIR}" "ge_start"
add_files_from_dir "${NEXT_DIR}" "lt_start"

[[ -s "${TMP_SELECTED}" ]] || { echo "Ошибка: не найдено файлов для обработки" >&2; exit 1; }

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

Если хочешь, я могу еще сразу дать версию, совместимую и с Linux, и с macOS, потому что там еще может отличаться `date -d` и `stat -c`.

----------
Измени способ добавления текста на кадры. Удали добавление субтитров (этот способ не работает сейчас) и сделай вариант с дополнительной отрисовкой на фото во временной папке.

возьми за основу скрипт:
echo "Start $(date '+%Y-%m-%d %H:%M:%S')" && \
SRCDIR='/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2026-01-23' && \
TMPDIR='/tmp/wm_photos_batch' && \
cd "$SRCDIR" && \
mkdir -p "$TMPDIR" && \
FONT="/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf" && \
\
echo "Обработка изображений..." && \
for f in *.jpg; do \
ffmpeg -y -i "$f" \
-vf "drawtext=fontfile=$FONT:text='$f':fontsize=24:fontcolor=white:x=(w-text_w)/2:y=h-th-10" \
-q:v 1 "$TMPDIR/$f" 2>/dev/null || { echo "Ошибка обработки $f"; exit 1; }; \
done && \
\
echo "Создание списка для concat..." && \
> "$TMPDIR/input.txt" && \
for f in "$TMPDIR"/*.jpg; do \
echo "file '$f'" >> "$TMPDIR/input.txt"; \
echo "duration 0.1" >> "$TMPDIR/input.txt"; \
done && \
\
echo "Сборка видео..." && \
ffmpeg -y -f concat -safe 0 -i "$TMPDIR/input.txt" -vf "format=yuv420p" "$SRCDIR/../timeline_2026-01-23_subs_10fps.mp4" && \
\
echo "Очистка..." && \
rm -rf "$TMPDIR" && \
echo "End $(date '+%Y-%m-%d %H:%M:%S')"

возьми от туда отрисовку, но временные файлы храни в нашей папке ./cache
удаляй временные файлы после работы

----------
Вот версия, где:

* **убран SRT/subtitles**
* текст **рисуется прямо на временных jpg**
* временные картинки кладутся в `./cache/...`
* после завершения временные файлы **удаляются**
* лог остается
* можно выбирать дату, fps и стартовое время
* на кадре показывается **имя файла без расширения**

```bash
#!/usr/bin/env bash

set -Eeuo pipefail

BASE_DIR="/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo"
FONT="/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"

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
TMP_SELECTED="${CACHE_DIR}/selected_${START_DATE}_${RUN_TS}.tmp"
TMP_IMG_DIR="${CACHE_DIR}/frames_${START_DATE}_${RUN_TS}"

OUTPUT_FILE="${HIST_DIR}/timeline_${NEXT_DATE}_${FPS}fps_from_${START_TIME//:/-}.mp4"

mkdir -p "${TMP_IMG_DIR}"

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
  echo "Cleanup temporary files..."
  rm -f "${LIST_FILE}" "${INPUT_FILE}" "${TMP_SELECTED}" 2>/dev/null || true
  rm -rf "${TMP_IMG_DIR}" 2>/dev/null || true
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
echo "Temporary img dir   : ${TMP_IMG_DIR}"
echo "Output file         : ${OUTPUT_FILE}"

[[ -d "${START_DIR}" ]] || { echo "Ошибка: папка не найдена: ${START_DIR}" >&2; exit 1; }
[[ -d "${NEXT_DIR}" ]] || { echo "Ошибка: папка следующего дня не найдена: ${NEXT_DIR}" >&2; exit 1; }
[[ -f "${FONT}" ]] || { echo "Ошибка: шрифт не найден: ${FONT}" >&2; exit 1; }

START_HHMMSS="$(to_hhmmss "${START_TIME}")"
[[ "${START_HHMMSS}" =~ ^[0-9]{6}$ ]] || { echo "Ошибка: время должно быть в формате HH:MM:SS" >&2; exit 1; }

: > "${TMP_SELECTED}"
: > "${LIST_FILE}"
: > "${INPUT_FILE}"

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

    printf '%s\n' "$f" >> "${TMP_SELECTED}"
    size="$(stat -c '%s' "$f")"
    selected_bytes=$((selected_bytes + size))
    selected_count=$((selected_count + 1))
  done < <(find "$dir" -maxdepth 1 -type f -name '*.jpg' -print0 | sort -z)
}

add_files_from_dir "${START_DIR}" "ge_start"
add_files_from_dir "${NEXT_DIR}" "lt_start"

[[ -s "${TMP_SELECTED}" ]] || { echo "Ошибка: не найдено файлов для обработки" >&2; exit 1; }

sort "${TMP_SELECTED}" > "${LIST_FILE}"

echo "Selected files count: ${selected_count}"
echo "Selected files size : $(human_size "${selected_bytes}")"
echo "Rendering text onto temporary images..."

idx=0
while IFS= read -r file; do
  idx=$((idx + 1))

  base="$(basename "$file")"
  label="$(strip_all_extensions "$base")"
  label_escaped="$(escape_drawtext "$label")"

  out_file="${TMP_IMG_DIR}/$(printf '%06d' "$idx")_${base}"

  echo "[$idx/$selected_count] $base"

  ffmpeg -y -loglevel error \
    -i "$file" \
    -vf "drawtext=fontfile=${FONT}:text='${label_escaped}':fontsize=24:fontcolor=white:x=(w-text_w)/2:y=h-th-10:box=1:boxcolor=black@0.5:boxborderw=8" \
    -q:v 1 \
    "$out_file" || { echo "Ошибка обработки $file" >&2; exit 1; }

  printf "file '%s'\n" "$out_file" >> "${INPUT_FILE}"
  printf "duration %.10f\n" "$(awk -v fps="$FPS" 'BEGIN { printf "%.10f", 1/fps }')" >> "${INPUT_FILE}"
done < "${LIST_FILE}"

last_frame="$(tail -n 1 "${INPUT_FILE}" | sed -n "s/^file '\(.*\)'$/\1/p")"
if [[ -n "${last_frame}" ]]; then
  printf "file '%s'\n" "${last_frame}" >> "${INPUT_FILE}"
fi

tmp_frames_bytes="$(du -sb "${TMP_IMG_DIR}" | awk '{print $1}')"

echo "Temporary rendered frames size: $(human_size "${tmp_frames_bytes}")"
echo "Building video..."

ffmpeg -y \
  -f concat -safe 0 -i "${INPUT_FILE}" \
  -vf "format=yuv420p" \
  "${OUTPUT_FILE}"

video_bytes="$(stat -c '%s' "${OUTPUT_FILE}")"

echo "Final video file    : ${OUTPUT_FILE}"
echo "Final video size    : $(human_size "${video_bytes}")"
echo "End $(date '+%Y-%m-%d %H:%M:%S')"
```

Как запускать:

```bash
chmod +x gen_timeline.sh
./gen_timeline.sh
```

Примеры:

```bash
./gen_timeline.sh 2026-03-09
./gen_timeline.sh 2026-03-09 12
./gen_timeline.sh 2026-03-09 12 06:00:00
./gen_timeline.sh 2026-03-09 8 05:30:00
```

Что делает по времени:

* из папки `START_DATE` берет файлы **от `START_TIME` и позже**
* из папки следующего дня берет файлы **раньше `START_TIME`**
* то есть окно ровно на сутки

Что сохраняется:

* итоговый mp4: `./historic/...`
* лог: `./cache/make_timeline_...log`

Что удаляется после работы:

* временный список файлов
* временные обработанные jpg в `./cache/frames_...`

Если хочешь, я могу еще сразу дать версию, где в надписи будет не только имя файла, но и отдельно красиво выделенное время кадра.
----------
./gen_timeline.sh: строка 172: printf: 0.1000000000: недопустимое число
----------
Да, это из-за строки:

```bash
printf "duration %.10f\n" "$(awk -v fps="$FPS" 'BEGIN { printf "%.10f", 1/fps }')" >> "${INPUT_FILE}"
```

На некоторых системах `bash printf` с таким аргументом работает криво. Надежнее вообще не давать дробное число в `printf` shell’а, а сразу писать строку через `awk` или обычный `%s`.

Замени этот кусок:

```bash
printf "file '%s'\n" "$out_file" >> "${INPUT_FILE}"
printf "duration %.10f\n" "$(awk -v fps="$FPS" 'BEGIN { printf "%.10f", 1/fps }')" >> "${INPUT_FILE}"
```

на такой:

```bash
printf "file '%s'\n" "$out_file" >> "${INPUT_FILE}"
printf "duration %s\n" "$(awk -v fps="$FPS" 'BEGIN { printf "%.10f", 1/fps }')" >> "${INPUT_FILE}"
```

Это самый простой фикс.

Еще лучше — один раз заранее посчитать длительность кадра и потом использовать:

```bash
FRAME_DURATION="$(awk -v fps="$FPS" 'BEGIN { printf "%.10f", 1/fps }')"
```

и в цикле уже так:

```bash
printf "file '%s'\n" "$out_file" >> "${INPUT_FILE}"
printf "duration %s\n" "$FRAME_DURATION" >> "${INPUT_FILE}"
```

То есть в скрипте добавь после проверки `FPS` и времени:

```bash
FRAME_DURATION="$(awk -v fps="$FPS" 'BEGIN { printf "%.10f", 1/fps }')"
```

А внутри цикла используй:

```bash
printf "file '%s'\n" "$out_file" >> "${INPUT_FILE}"
printf "duration %s\n" "$FRAME_DURATION" >> "${INPUT_FILE}"
```

Если хочешь, вот готовый исправленный фрагмент вокруг нужного места:

```bash
START_HHMMSS="$(to_hhmmss "${START_TIME}")"
[[ "${START_HHMMSS}" =~ ^[0-9]{6}$ ]] || { echo "Ошибка: время должно быть в формате HH:MM:SS" >&2; exit 1; }
[[ "${FPS}" =~ ^[0-9]+$ ]] || { echo "Ошибка: FPS должен быть целым положительным числом" >&2; exit 1; }
[[ "${FPS}" -gt 0 ]] || { echo "Ошибка: FPS должен быть больше 0" >&2; exit 1; }

FRAME_DURATION="$(awk -v fps="$FPS" 'BEGIN { printf "%.10f", 1/fps }')"

: > "${TMP_SELECTED}"
: > "${LIST_FILE}"
: > "${INPUT_FILE}"
```

И внутри цикла:

```bash
  ffmpeg -y -loglevel error \
    -i "$file" \
    -vf "drawtext=fontfile=${FONT}:text='${label_escaped}':fontsize=24:fontcolor=white:x=(w-text_w)/2:y=h-th-10:box=1:boxcolor=black@0.5:boxborderw=8" \
    -q:v 1 \
    "$out_file" || { echo "Ошибка обработки $file" >&2; exit 1; }

  printf "file '%s'\n" "$out_file" >> "${INPUT_FILE}"
  printf "duration %s\n" "$FRAME_DURATION" >> "${INPUT_FILE}"
```

Если после этого будет ошибка, тогда уже дам полностью финальную версию скрипта целиком с этими правками.

----------

Напиши исправленный файл целиком

----------

Вот исправленный файл целиком.

```bash
#!/usr/bin/env bash

set -Eeuo pipefail

BASE_DIR="/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo"
FONT="/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"

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
TMP_SELECTED="${CACHE_DIR}/selected_${START_DATE}_${RUN_TS}.tmp"
TMP_IMG_DIR="${CACHE_DIR}/frames_${START_DATE}_${RUN_TS}"

OUTPUT_FILE="${HIST_DIR}/timeline_${NEXT_DATE}_${FPS}fps_from_${START_TIME//:/-}.mp4"

mkdir -p "${TMP_IMG_DIR}"

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
  echo "Cleanup temporary files..."
  rm -f "${LIST_FILE}" "${INPUT_FILE}" "${TMP_SELECTED}" 2>/dev/null || true
  rm -rf "${TMP_IMG_DIR}" 2>/dev/null || true
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
echo "Temporary img dir   : ${TMP_IMG_DIR}"
echo "Output file         : ${OUTPUT_FILE}"

[[ -d "${START_DIR}" ]] || { echo "Ошибка: папка не найдена: ${START_DIR}" >&2; exit 1; }
[[ -d "${NEXT_DIR}" ]] || { echo "Ошибка: папка следующего дня не найдена: ${NEXT_DIR}" >&2; exit 1; }
[[ -f "${FONT}" ]] || { echo "Ошибка: шрифт не найден: ${FONT}" >&2; exit 1; }

START_HHMMSS="$(to_hhmmss "${START_TIME}")"
[[ "${START_HHMMSS}" =~ ^[0-9]{6}$ ]] || { echo "Ошибка: время должно быть в формате HH:MM:SS" >&2; exit 1; }
[[ "${FPS}" =~ ^[0-9]+$ ]] || { echo "Ошибка: FPS должен быть целым положительным числом" >&2; exit 1; }
[[ "${FPS}" -gt 0 ]] || { echo "Ошибка: FPS должен быть больше 0" >&2; exit 1; }

FRAME_DURATION="$(awk -v fps="$FPS" 'BEGIN { printf "%.10f", 1/fps }')"

: > "${TMP_SELECTED}"
: > "${LIST_FILE}"
: > "${INPUT_FILE}"

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

    printf '%s\n' "$f" >> "${TMP_SELECTED}"
    size="$(stat -c '%s' "$f")"
    selected_bytes=$((selected_bytes + size))
    selected_count=$((selected_count + 1))
  done < <(find "$dir" -maxdepth 1 -type f -name '*.jpg' -print0 | sort -z)
}

add_files_from_dir "${START_DIR}" "ge_start"
add_files_from_dir "${NEXT_DIR}" "lt_start"

[[ -s "${TMP_SELECTED}" ]] || { echo "Ошибка: не найдено файлов для обработки" >&2; exit 1; }

sort "${TMP_SELECTED}" > "${LIST_FILE}"

echo "Selected files count: ${selected_count}"
echo "Selected files size : $(human_size "${selected_bytes}")"
echo "Rendering text onto temporary images..."

idx=0
while IFS= read -r file; do
  idx=$((idx + 1))

  base="$(basename "$file")"
  label="$(strip_all_extensions "$base")"
  label_escaped="$(escape_drawtext "$label")"

  out_file="${TMP_IMG_DIR}/$(printf '%06d' "$idx")_${base}"

  echo "[$idx/$selected_count] $base"

  ffmpeg -y -loglevel error \
    -i "$file" \
    -vf "drawtext=fontfile=${FONT}:text='${label_escaped}':fontsize=24:fontcolor=white:x=(w-text_w)/2:y=h-th-10:box=1:boxcolor=black@0.5:boxborderw=8" \
    -q:v 1 \
    "$out_file" || { echo "Ошибка обработки $file" >&2; exit 1; }

  printf "file '%s'\n" "$out_file" >> "${INPUT_FILE}"
  printf "duration %s\n" "$FRAME_DURATION" >> "${INPUT_FILE}"
done < "${LIST_FILE}"

last_frame="$(sed -n '$p' "${INPUT_FILE}" | sed -n "s/^file '\(.*\)'$/\1/p")"
if [[ -n "${last_frame}" ]]; then
  printf "file '%s'\n" "${last_frame}" >> "${INPUT_FILE}"
fi

tmp_frames_bytes="$(du -sb "${TMP_IMG_DIR}" | awk '{print $1}')"

echo "Temporary rendered frames size: $(human_size "${tmp_frames_bytes}")"
echo "Building video..."

ffmpeg -y \
  -f concat -safe 0 -i "${INPUT_FILE}" \
  -vf "format=yuv420p" \
  "${OUTPUT_FILE}"

video_bytes="$(stat -c '%s' "${OUTPUT_FILE}")"

echo "Final video file    : ${OUTPUT_FILE}"
echo "Final video size    : $(human_size "${video_bytes}")"
echo "End $(date '+%Y-%m-%d %H:%M:%S')"
```

Сохранить можно так:

```bash
nano gen_timeline.sh
```

потом вставить, сохранить и дать права:

```bash
chmod +x gen_timeline.sh
```

Запуск:

```bash
./gen_timeline.sh
./gen_timeline.sh 2026-03-09
./gen_timeline.sh 2026-03-09 10
./gen_timeline.sh 2026-03-09 10 06:00:00
```

Если у тебя окажется, что `du -sb` не поддерживается, замени строку:

```bash
tmp_frames_bytes="$(du -sb "${TMP_IMG_DIR}" | awk '{print $1}')"
```

на:

```bash
tmp_frames_bytes="$(find "${TMP_IMG_DIR}" -type f -printf '%s\n' | awk '{s+=$1} END {print s+0}')"
```

И еще один момент: сейчас скрипт строго требует, чтобы папка следующего дня существовала. Если хочешь, я могу сразу дать версию, где это необязательно, и он просто берет файлы только из первой папки, если второй нет.


----------


Что-то не так, я не вижу, чтобы файлы появлялись во временной папке и потом сам скрипт не выдает никакое видео и есть такой лог с ошибкой:

Rendering text onto temporary images...
[1/28470] front_cam20260311_024602.jpg.jpg
[2/28470] front_cam20260311_024605.jpg.jpg
home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2026-03-11/front_cam20260311_024605.jpg.jpg: No such file or directory
Ошибка обработки home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2026-03-11/front_cam20260311_024605.jpg.jpg

----------

Да, причина похожа на работу с **неабсолютным путем**.

По логу видно, что в ошибке путь такой:

```text
home/resager/rudb/...
```

а должен быть:

```text
/home/resager/rudb/...
```

То есть где-то путь попадает в обработку **без начального `/`**. Из-за этого `ffmpeg` ищет файл относительно текущей директории.

И еще второй момент: ты не успеваешь увидеть временные файлы, потому что у нас стоит `trap cleanup EXIT`, и при ошибке скрипт сразу удаляет `./cache/frames_...`.

Ниже даю исправленную версию целиком. Что я поменял:

* принудительно нормализую пути в абсолютные
* временные файлы **не удаляются при ошибке**
* удаляются только при успешном завершении
* в лог пишется полный путь входного и выходного файла
* добавил `KEEP_CACHE_ON_SUCCESS=0`, можно поменять на `1`, если хочешь оставлять кэш и после успешной сборки

---

```bash
#!/usr/bin/env bash

set -Eeuo pipefail

BASE_DIR="/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo"
FONT="/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"

START_DATE="${1:-$(date -d 'yesterday' +%F)}"
FPS="${2:-10}"
START_TIME="${3:-06:00:00}"

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
TMP_SELECTED="${CACHE_DIR}/selected_${START_DATE}_${RUN_TS}.tmp"
TMP_IMG_DIR="${CACHE_DIR}/frames_${START_DATE}_${RUN_TS}"

OUTPUT_FILE="${HIST_DIR}/timeline_${NEXT_DATE}_${FPS}fps_from_${START_TIME//:/-}.mp4"

mkdir -p "${TMP_IMG_DIR}"

exec > >(tee -a "${LOG_FILE}") 2>&1

SUCCESS=0

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
    rm -f "${LIST_FILE}" "${INPUT_FILE}" "${TMP_SELECTED}" 2>/dev/null || true
    rm -rf "${TMP_IMG_DIR}" 2>/dev/null || true
  else
    echo "Temporary files kept:"
    echo "  LIST_FILE   = ${LIST_FILE}"
    echo "  INPUT_FILE  = ${INPUT_FILE}"
    echo "  TMP_SELECTED= ${TMP_SELECTED}"
    echo "  TMP_IMG_DIR = ${TMP_IMG_DIR}"
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
[[ "${START_HHMMSS}" =~ ^[0-9]{6}$ ]] || { echo "Ошибка: время должно быть в формате HH:MM:SS" >&2; exit 1; }
[[ "${FPS}" =~ ^[0-9]+$ ]] || { echo "Ошибка: FPS должен быть целым положительным числом" >&2; exit 1; }
[[ "${FPS}" -gt 0 ]] || { echo "Ошибка: FPS должен быть больше 0" >&2; exit 1; }

FRAME_DURATION="$(awk -v fps="$FPS" 'BEGIN { printf "%.10f", 1/fps }')"

: > "${TMP_SELECTED}"
: > "${LIST_FILE}"
: > "${INPUT_FILE}"

selected_count=0
selected_bytes=0

add_files_from_dir() {
  local dir="$1"
  local mode="$2"
  local f base hhmmss size absf

  while IFS= read -r -d '' f; do
    absf="$(readlink -f "$f" 2>/dev/null || realpath "$f" 2>/dev/null || printf '%s' "$f")"
    base="$(basename "$absf")"

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

    printf '%s\n' "$absf" >> "${TMP_SELECTED}"
    size="$(stat -c '%s' "$absf")"
    selected_bytes=$((selected_bytes + size))
    selected_count=$((selected_count + 1))
  done < <(find "$dir" -maxdepth 1 -type f -name '*.jpg' -print0 | sort -z)
}

add_files_from_dir "${START_DIR}" "ge_start"
add_files_from_dir "${NEXT_DIR}" "lt_start"

[[ -s "${TMP_SELECTED}" ]] || { echo "Ошибка: не найдено файлов для обработки" >&2; exit 1; }

sort "${TMP_SELECTED}" > "${LIST_FILE}"

echo "Selected files count: ${selected_count}"
echo "Selected files size : $(human_size "${selected_bytes}")"
echo "Rendering text onto temporary images..."

idx=0
while IFS= read -r file; do
  idx=$((idx + 1))

  [[ -f "$file" ]] || { echo "Ошибка: исходный файл не существует: $file" >&2; exit 1; }

  base="$(basename "$file")"
  label="$(strip_all_extensions "$base")"
  label_escaped="$(escape_drawtext "$label")"

  out_file="${TMP_IMG_DIR}/$(printf '%06d' "$idx")_${base}"

  echo "[$idx/$selected_count]"
  echo "  input : $file"
  echo "  output: $out_file"

  ffmpeg -y -loglevel error \
    -i "$file" \
    -vf "drawtext=fontfile=${FONT}:text='${label_escaped}':fontsize=24:fontcolor=white:x=(w-text_w)/2:y=h-th-10:box=1:boxcolor=black@0.5:boxborderw=8" \
    -q:v 1 \
    "$out_file" || { echo "Ошибка обработки $file" >&2; exit 1; }

  [[ -f "$out_file" ]] || { echo "Ошибка: временный файл не создан: $out_file" >&2; exit 1; }

  printf "file '%s'\n" "$out_file" >> "${INPUT_FILE}"
  printf "duration %s\n" "$FRAME_DURATION" >> "${INPUT_FILE}"
done < "${LIST_FILE}"

last_frame="$(find "${TMP_IMG_DIR}" -maxdepth 1 -type f -name '*.jpg' | sort | tail -n 1)"
if [[ -n "${last_frame}" ]]; then
  printf "file '%s'\n" "${last_frame}" >> "${INPUT_FILE}"
fi

tmp_frames_bytes="$(find "${TMP_IMG_DIR}" -type f -printf '%s\n' | awk '{s+=$1} END {print s+0}')"

echo "Temporary rendered frames size: $(human_size "${tmp_frames_bytes}")"
echo "Building video..."

ffmpeg -y \
  -f concat -safe 0 -i "${INPUT_FILE}" \
  -vf "format=yuv420p" \
  "${OUTPUT_FILE}"

[[ -f "${OUTPUT_FILE}" ]] || { echo "Ошибка: итоговое видео не создано" >&2; exit 1; }

video_bytes="$(stat -c '%s' "${OUTPUT_FILE}")"

echo "Final video file    : ${OUTPUT_FILE}"
echo "Final video size    : $(human_size "${video_bytes}")"
echo "End $(date '+%Y-%m-%d %H:%M:%S')"

SUCCESS=1
```

---

Что проверить сразу после замены:

1. Запусти так:

```bash
bash ./gen_timeline.sh 2026-03-10 10 06:00:00
```

2. Если снова будет ошибка, в логе теперь должны быть строки вида:

```text
input : /home/resager/...
output: /.../cache/frames_.../000001_...
```

3. Если хочешь увидеть временные jpg даже после успеха, поменяй:

```bash
KEEP_CACHE_ON_SUCCESS=0
```

на:

```bash
KEEP_CACHE_ON_SUCCESS=1
```

Самая вероятная причина сейчас — именно путь без начального `/` и мгновенная очистка кэша при падении. Если хочешь, следующим сообщением я могу дать еще более быстрый вариант, где не создаются десятки тысяч промежуточных jpg, а текст рисуется пакетно через `ffmpeg` по списку кадров.

----------

я сделал KEEP_CACHE_ON_SUCCESS=1 и увидел всего 1файл
видеофайл результирующий не был создан,  лог примерно такой:
Log file            : /home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/cache/make_timeline_2026-03-11_20260312_095611.log
Start date          : 2026-03-11
Next date           : 2026-03-12
FPS                 : 10
Start time          : 06:00:00
Base dir            : /home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo
Start dir           : /home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2026-03-11
Next dir            : /home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2026-03-12
Temporary img dir   : /home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/cache/frames_2026-03-11_20260312_095611
Output file         : /home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/historic/timeline_2026-03-12_10fps_from_06-00-00.mp4
Selected files count: 28470
Selected files size : 1.82 GB
Rendering text onto temporary images...
[1/28470]
input : /home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2026-03-11/front_cam20260311_024602.jpg.jpg
output: /home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/cache/frames_2026-03-11_20260312_095611/000001_front_cam20260311_024602.jpg.jpg
Ошибка: исходный файл не существует: home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2026-03-11/front_cam20260311_024605.jpg.jpg
Temporary files kept:
LIST_FILE   = /home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/cache/list_2026-03-11_20260312_095611.txt
INPUT_FILE  = /home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/cache/input_2026-03-11_20260312_095611.txt
TMP_SELECTED= /home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/cache/selected_2026-03-11_20260312_095611.tmp
TMP_IMG_DIR = /home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/cache/frames_2026-03-11_20260312_095611

попробуй исправить это и пришли полный файл

----------
Проблема была не в `ffmpeg`, а в логике подготовки списка файлов: где-то по пути часть значений начинала жить уже не как гарантированно абсолютный путь.
Я переписал скрипт так, чтобы:

* вообще не зависеть от `find ... | sort ... | read ...` для основного списка
* собирать список файлов через `bash`-glob из каждой папки
* явно строить абсолютный путь как `"$dir/$filename"`
* рисовать текст на временные JPG в `./cache/...`
* сохранять лог
* удалять временные файлы только при успехе, а при ошибке оставлять
* корректно собирать concat-список для `ffmpeg`

Вот полный исправленный файл.

```bash
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
KEEP_CACHE_ON_SUCCESS=1

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

  echo "[$idx/$selected_count]"
  echo "  input : $file"
  echo "  output: $out_file"

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
```

Что изменилось по сути:

* убрал проблемную цепочку с `find -print0 | sort -z | while read`
* теперь второй файл уже не должен превращаться в `home/...` без `/`
* временные кадры должны создаваться последовательно: `000001_...`, `000002_...` и так далее

Запусти так:

```bash
bash ./gen_timeline.sh 2026-03-11 10 06:00:00
```

Если ошибка повторится, то теперь она уже почти наверняка будет связана либо с конкретным битым JPG, либо с `ffmpeg`, а не с потерей `/` в пути.

----------

Перенеси всю логику на golang приложение + (если так будет быстрее), перенеси  нанесение текста на фото так же на библиотеку внутри golang приложения, вместо ffmpeg. Пусть только видео теперь генерирует ffmpeg, остальным занимается приложение

----------


Готово — вынес логику в Go-файл рядом в canvas: `photo_timeline_generator.go`.

Что делает приложение:

* выбирает файлы из папки стартовой даты от заданного времени
* добирает файлы из следующей даты до этого же времени
* рисует текст на кадры **внутри Go**, без `ffmpeg drawtext`
* складывает временные jpg в `./cache`
* считает размер исходных и временных файлов
* вызывает `ffmpeg` только на финальной сборке mp4
* пишет лог

Что нужно для сборки:

```bash
go mod init photo-timeline
go get golang.org/x/image/font golang.org/x/image/font/opentype golang.org/x/image/math/fixed
go build -o photo-timeline photo_timeline_generator.go
```

Запуск:

```bash
./photo-timeline -date 2026-03-11 -fps 10 -start-time 06:00:00
```

Полезные флаги:

```bash
-base-dir /home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo
-cache-dir ./cache
-historic-dir ./historic
-font /usr/share/fonts/truetype/dejavu/DejaVuSans.ttf
-workers 8
-jpeg-quality 92
-keep-cache=true
-ffmpeg ffmpeg
```

Пример:

```bash
./photo-timeline \
  -date 2026-03-11 \
  -fps 10 \
  -start-time 06:00:00 \
  -workers 8 \
  -keep-cache=true
```

Пара замечаний:

* Сейчас приложение требует, чтобы папка следующего дня существовала.
* Подпись рисуется как белый текст на полупрозрачном черном прямоугольнике снизу по центру.
* Если системный шрифт не найдется, используется встроенный Go-шрифт.

Я бы еще сразу поправил две вещи в следующей итерации:

1. ограничение длины подписи, если имя файла очень длинное;
2. сохранение прогресса не на каждый кадр, а, например, раз в 100 кадров — так лог будет легче.


----------

ошибка


2026/03/12 21:36:53 [68/28484] 000066_front_cam20260308_060317.jpg.jpg
2026/03/12 21:36:53 [69/28484] 000069_front_cam20260308_060326.jpg.jpg
panic: runtime error: index out of range [201] with length 42

goroutine 20 [running]:
image/draw.drawGlyphOver(0xc0001f2100, {{0xc0001f2100?, 0xc000231b68?}, {0x5aea10?, 0xc00018e020?}}, 0xc000231b28?, 0xc000119bb0, {0xc000119bb0?, 0xc000231b48?})
/home/resager/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.7.linux-amd64/src/image/draw/draw.go:624 +0x267
image/draw.DrawMask({0x5aed40, 0xc0001f2100}, {{0x204, 0x392}, {0x211, 0x3a4}}, {0x5aea10, 0xc00018e020}, {0x0, 0x0}, ...)
/home/resager/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.7.linux-amd64/src/image/draw/draw.go:166 +0x2af
golang.org/x/image/font.(*Drawer).DrawString(0xc000231d80, {0xc000367e20, 0x18})
/home/resager/go/pkg/mod/golang.org/x/image@v0.37.0/font/font.go:180 +0x16b
main.renderSingleFrame({0xc000442c80, 0x74}, {0xc0006b84d0, 0xa6}, {0xc000367e20, 0x18}, {0x5af308, 0xc000119508}, 0x5c)
/home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/photo_timeline_generator.go:408 +0x609
main.renderFrames.func1()
/home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/photo_timeline_generator.go:325 +0x2f4
created by main.renderFrames in goroutine 1
/home/resager/rudb/mount/2tb-ext-part/rudb/dev/scripts/scripts/photo-to-video-timeline/photo_timeline_generator.go:321 +0x170

----------


Да, причина почти наверняка в том, что один и тот же `font.Face` использовался одновременно из нескольких goroutine. Это не потокобезопасно, и отсюда panic внутри `image/draw`.

Я уже поправил код:

* теперь **каждый worker создает свой собственный `font.Face`**
* общий `font.Face` между goroutine больше не шарится
* это должно убрать `index out of range` в `drawGlyphOver`

Что сделать у себя:

```bash
go build -o photo-timeline photo_timeline_generator.go
./photo-timeline -date 2026-03-11 -fps 10 -start-time 06:00:00 -workers 8 -keep-cache=true
```

Если хочешь максимально безопасно проверить, сначала запусти вообще в один worker:

```bash
./photo-timeline -date 2026-03-11 -fps 10 -start-time 06:00:00 -workers 1 -keep-cache=true
```

Если с `-workers 1` все пройдет, а с `-workers 8` тоже теперь будет ок — значит фикс сработал как надо.

Еще один практичный момент: у тебя кадров очень много, так что лог на каждый кадр может быть тяжеловат. Следующим шагом я бы еще упростил логирование, чтобы писать прогресс раз в 100 или 500 кадров.

----------

