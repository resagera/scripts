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
