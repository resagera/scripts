как сделать, чтобы в команде
echo "Start $(date '+%Y-%m-%d %H:%M:%S')" && cd '/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2025-12-27' && ls *.jpg | sort > list.txt && sed 's/^/file /' list.txt > input.txt && ffmpeg -f concat -safe 0 -i input.txt -vf "fps=10,format=yuv420p" ../timeline_2025-12-28_10fps.mp4 && echo "End $(date '+%Y-%m-%d %H:%M:%S')"
на результирующем видео в каждом кадре проставлялось имя файла?

----------
Прямое наложение имени файла на каждый кадр при использовании `-f concat` (демультиплексора конкатенации) в одной команде **невозможно**.

**Почему:** Когда вы используете `-f concat`, FFmpeg объединяет все файлы в один непрерывный поток *до* применения фильтров. Для фильтра `drawtext` это выглядит как одно длинное видео, и он не знает, где заканчивается один файл и начинается другой, поэтому не может динамически менять текст.

**Решение:** Нужно сделать это в два этапа:
1.  Пройтись циклом по картинкам и "вжечь" (burn-in) имя файла в каждую из них (сохранить во временную папку).
2.  Собрать видео уже из обработанных картинок.

Вот модифицированный вариант вашей команды. Я добавил создание временной папки, цикл обработки и очистку в конце.

```bash
echo "Start $(date '+%Y-%m-%d %H:%M:%S')" && \
cd '/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2025-12-27' && \
mkdir -p /tmp/wm_photos && \
FONT="/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf" && \
for f in *.jpg; do \
  ffmpeg -y -i "$f" -vf "drawtext=fontfile=$FONT:text='$f':fontsize=24:fontcolor=white:x=(w-text_w)/2:y=h-th-10" -q:v 1 "/tmp/wm_photos/$f" 2>/dev/null; \
done && \
( for f in /tmp/wm_photos/*.jpg; do echo "file '$PWD/$f'"; echo "duration 0.1"; done; echo "file '$PWD/$f'" ) > input_wm.txt && \
ffmpeg -y -f concat -safe 0 -i input_wm.txt -vf "format=yuv420p" ../timeline_2025-12-28_10fps.mp4 && \
rm -rf /tmp/wm_photos input_wm.txt && \
echo "End $(date '+%Y-%m-%d %H:%M:%S')"
```

### Что изменено и на что обратить внимание:

1.  **Цикл `for`**: Мы проходим по каждому `.jpg`, запускаем `ffmpeg` с фильтром `drawtext`, который пишет имя файла (`text='$f'`) внизу по центру. Обработанные файлы летят в `/tmp/wm_photos`.
2.  **Шрифт (`fontfile`)**: Вам нужно указать путь к реальному шрифту в вашей системе.
    *   В примере указан стандартный путь для Debian/Ubuntu (`/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf`).
    *   Если у вас другой Linux или macOS, найдите шрифт командой `find /usr -name "*.ttf"` и замените путь в переменной `FONT`.
3.  **Длительность кадра (`duration 0.1`)**: В оригинале вы использовали `fps=10` на выходе, но при `-f concat` картинки по умолчанию держатся 5 секунд. Я добавил генерацию списка `input_wm.txt` с директивой `duration 0.1`, чтобы каждая картинка длилась 0.1 секунды (что в сумме дает 10 кадров в секунду для таймлапса).
4.  **`2>/dev/null`**: В цикле я скрыл логи FFmpeg, чтобы консоль не засорялась сотнями строк на каждую картинку. Ошибки всё равно прервут выполнение из-за `&&`.
5.  **Очистка**: В конце удаляется временная папка и список файлов.

### Если картинок очень много (производительность)
Этот метод запускает FFmpeg **отдельно для каждой картинки**. Если у вас 1000 фото, это будет 1000 запусков FFmpeg, что может занять много времени.
Если это критично, единственная быстрая альтернатива — использовать `concat` фильтр внутри одного запуска FFmpeg (через `-i img1 -i img2 ... -filter_complex`), но это упрется в лимит длины аргументов командной строки, если файлов больше ~100-200. Для больших объемов предложенный выше скрипт — самый надежный вариант.

----------
[concat @ 0x597dc81a0240] Impossible to open '/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2026-01-23//tmp/wm_photos/front_cam20260123_120542.jpg.jpg'
input_wm.txt: No such file or directory


----------

Ошибка возникает из-за двух проблем в предыдущей команде:

1.  **Неверная склейка путей**: В конструкции `echo "file '$PWD/$f'"` переменная `$f` уже содержит абсолютный путь (так как берется из `/tmp/wm_photos/*.jpg`). При добавлении `$PWD` в начале путь ломается: `/текущая/папка//tmp/...`, что FFmpeg не может прочитать.
2.  **Дублирование расширения**: Ошибка `...jpg.jpg` в логе говорит о том, что исходные файлы, скорее всего, уже имеют расширение `.jpg`, и при сохранении во временную папку оно добавляется повторно.

Вот исправленная и более надежная версия скрипта. Я вынес пути в переменные и упростил логику создания списка файлов.

```bash
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
ffmpeg -y -f concat -safe 0 -i "$TMPDIR/input.txt" -vf "format=yuv420p" "$SRCDIR/../timeline_2026-01-23_10fps.mp4" && \
\
echo "Очистка..." && \
rm -rf "$TMPDIR" && \
echo "End $(date '+%Y-%m-%d %H:%M:%S')"
```

### Что исправлено:

1.  **Переменные `SRCDIR` и `TMPDIR`**: Позволяют избежать дублирования длинных путей и ошибок при их написании.
2.  **Абсолютные пути в списке**: Цикл `for f in "$TMPDIR"/*.jpg` сразу подставляет полный путь (например, `/tmp/.../img.jpg`), поэтому в файл `input.txt` мы пишем его "как есть", без добавления `$PWD`.
3.  **Безопасная запись списка**: Используется явное перенаправление `>>` в цикле, что надежнее, чем конвейеры или подстановки, которые могут обрезаться.
4.  **Проверка ошибок**: Конструкция `|| { echo ...; exit 1; }` остановит скрипт, если обработка хотя бы одного кадра упадет, чтобы не собирать битое видео.
5.  **Имя выходного файла**: Исправлено на `timeline_2026-01-23_10fps.mp4` в соответствии с датой в пути (`2026-01-23`).

### Важные примечания:

*   **Шрифт**: Убедитесь, что файл шрифта существует по пути `/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf`. Если нет — найдите любой `.ttf` файл в системе командой `find /usr -name "*.ttf"` и замените путь в переменной `FONT`.
*   **Расширения файлов**: Скрипт ожидает, что исходные файлы называются `*.jpg`. Если у вас `.jpeg` или `.JPG`, измените маску в цикле `for f in *.jpg`.
*   **Тест**: Перед запуском на всей папке рекомендую создать тестовую директорию с 3-5 фото и прогнать скрипт там, чтобы убедиться в корректности путей и шрифта.

----------

ffmpeg достаточно быстро рисует текст на фото? Быть может быстрее будет это делать imageMagic?

----------


Короткий ответ: **Для вашей задачи (наложение текста на отдельные JPG) ImageMagick, скорее всего, будет немного быстрее и проще**, но разница не будет колоссальной. Главное преимущество — отсутствие накладных расходов на декодирование/кодирование видео-контейнера, которые есть у FFmpeg даже при работе с одним кадром.

### Сравнение производительности

| Критерий | FFmpeg (`drawtext`) | ImageMagick (`convert`/`magick`) |
|----------|-------------------|--------------------------------|
| **Скорость на 1 фото** | ~0.3–0.8 сек | ~0.1–0.4 сек [[13]] |
| **Накладные расходы** | Высокие (инициализация кодеков, фильтров) | Низкие (прямая работа с памятью) |
| **Качество текста** | Отличное (библиотека FreeType) | Отличное (та же библиотека) |
| **Потребление памяти** | Выше (видео-пайплайн) | Ниже (растровая обработка) [[19]] |
| **Настройка шрифтов** | Требует `fontfile=`, чувствителен к путям | Проще: ` -font Sans` или путь к .ttf |

> ⚠️ **Важно**: ImageMagick известен проблемами с производительностью при неправильной конфигурации (policy.xml, лимиты памяти) [[16]][[19]]. Если он «тормозит» — проверьте настройки.

---

### Вариант 1: ImageMagick (рекомендуемый для вашей задачи)

```bash
echo "Start $(date '+%Y-%m-%d %H:%M:%S')" && \
SRCDIR='/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo/2026-01-23' && \
TMPDIR='/tmp/wm_photos_batch' && \
cd "$SRCDIR" && \
mkdir -p "$TMPDIR" && \
\
echo "Обработка изображений (ImageMagick)..." && \
for f in *.jpg; do \
  magick "$f" \
    -gravity South \
    -pointsize 24 \
    -fill white \
    -annotate +0+10 "$f" \
    -quality 95 \
    "$TMPDIR/$f" || { echo "Ошибка: $f"; exit 1; }; \
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
ffmpeg -y -f concat -safe 0 -i "$TMPDIR/input.txt" -vf "format=yuv420p" "$SRCDIR/../timeline_2026-01-23_10fps.mp4" && \
\
echo "Очистка..." && \
rm -rf "$TMPDIR" && \
echo "End $(date '+%Y-%m-%d %H:%M:%S')"
```

**Преимущества этого подхода:**
- `magick` (или `convert` в старых версиях) работает напрямую с растром, без видео-контекста
- Параметр `-gravity South` автоматически центрирует текст по горизонтали внизу
- `-annotate +0+10` — отступ 10 пикселей от нижнего края
- Меньше шансов на ошибки с путями к шрифтам

---

### Вариант 2: Если нужен максимальный параллелизм

Если у вас **тысячи фото**, можно ускорить обработку, распараллелив цикл:

```bash
# Используем GNU Parallel (установите: sudo apt install parallel)
export SRCDIR TMPDIR FONT
cd "$SRCDIR" && mkdir -p "$TMPDIR"

# Обработка в 4 потока
parallel -j 4 '
  magick {} \
    -gravity South -pointsize 24 -fill white -annotate +0+10 {/} \
    -quality 95 "'"$TMPDIR"'/{/}"
' ::: *.jpg
```

> 📊 По тестам, GraphicsMagick (форк ImageMagick) может быть **в разы быстрее** на некоторых операциях [[7]][[12]], но для `annotate` разница обычно не критична.

---

### Как проверить скорость на вашей системе

Запустите мини-бенчмарк перед обработкой всей папки:

```bash
# Тест FFmpeg
time ffmpeg -y -i test.jpg -vf "drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf:text='test.jpg':fontsize=24:fontcolor=white:x=(w-text_w)/2:y=h-th-10" -q:v 1 /tmp/out_ffmpeg.jpg -frames:v 1

# Тест ImageMagick  
time magick test.jpg -gravity South -pointsize 24 -fill white -annotate +0+10 "test.jpg" -quality 95 /tmp/out_im.jpg

# Сравните время в выводе `time`
```

---

### Итоговая рекомендация

1.  **Попробуйте ImageMagick** — он проще в настройке для статичных изображений и часто быстрее на 20–40% [[13]][[19]].
2.  **Убедитесь, что установлен `imagemagick`** (в Debian/Ubuntu: `sudo apt install imagemagick`).
3.  **Если фото >500 шт.** — добавьте `parallel` для многопоточности.
4.  **Если нужна анимация текста** (появление, движение) — тогда возвращайтесь к FFmpeg, но уже на этапе сборки видео, а не обработки фото.

Нужно помочь адаптировать скрипт под ImageMagick или настроить параллельную обработку?

----------
$time ffmpeg -y -i test.jpg -vf "drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf:text='test.jpg':fontsize=24:fontcolor=white:x=(w-text_w)/2:y=h-th-10" -q:v 1 /tmp/out_ffmpeg.jpg -frames:v 1
ffmpeg version 4.4.2-0ubuntu0.22.04.1 Copyright (c) 2000-2021 the FFmpeg developers
built with gcc 11 (Ubuntu 11.2.0-19ubuntu1)
configuration: --prefix=/usr --extra-version=0ubuntu0.22.04.1 --toolchain=hardened --libdir=/usr/lib/x86_64-linux-gnu --incdir=/usr/include/x86_64-linux-gnu --arch=amd64 --enable-gpl --disable-stripping --enable-gnutls --enable-ladspa --enable-libaom --enable-libass --enable-libbluray --enable-libbs2b --enable-libcaca --enable-libcdio --enable-libcodec2 --enable-libdav1d --enable-libflite --enable-libfontconfig --enable-libfreetype --enable-libfribidi --enable-libgme --enable-libgsm --enable-libjack --enable-libmp3lame --enable-libmysofa --enable-libopenjpeg --enable-libopenmpt --enable-libopus --enable-libpulse --enable-librabbitmq --enable-librubberband --enable-libshine --enable-libsnappy --enable-libsoxr --enable-libspeex --enable-libsrt --enable-libssh --enable-libtheora --enable-libtwolame --enable-libvidstab --enable-libvorbis --enable-libvpx --enable-libwebp --enable-libx265 --enable-libxml2 --enable-libxvid --enable-libzimg --enable-libzmq --enable-libzvbi --enable-lv2 --enable-omx --enable-openal --enable-opencl --enable-opengl --enable-sdl2 --enable-pocketsphinx --enable-librsvg --enable-libmfx --enable-libdc1394 --enable-libdrm --enable-libiec61883 --enable-chromaprint --enable-frei0r --enable-libx264 --enable-shared
libavutil      56. 70.100 / 56. 70.100
libavcodec     58.134.100 / 58.134.100
libavformat    58. 76.100 / 58. 76.100
libavdevice    58. 13.100 / 58. 13.100
libavfilter     7.110.100 /  7.110.100
libswscale      5.  9.100 /  5.  9.100
libswresample   3.  9.100 /  3.  9.100
libpostproc    55.  9.100 / 55.  9.100
Trailing option(s) found in the command: may be ignored.
Input #0, image2, from 'test.jpg':
Duration: 00:00:00.04, start: 0.000000, bitrate: 16476 kb/s
Stream #0:0: Video: mjpeg (Baseline), yuvj420p(pc, bt470bg/unknown/unknown), 720x960 [SAR 1:1 DAR 3:4], 25 fps, 25 tbr, 25 tbn, 25 tbc
Stream mapping:
Stream #0:0 -> #0:0 (mjpeg (native) -> mjpeg (native))
Press [q] to stop, [?] for help
Output #0, image2, to '/tmp/out_ffmpeg.jpg':
Metadata:
encoder         : Lavf58.76.100
Stream #0:0: Video: mjpeg, yuvj420p(pc, bt470bg/unknown/unknown, progressive), 720x960 [SAR 1:1 DAR 3:4], q=2-31, 200 kb/s, 25 fps, 25 tbn
Metadata:
encoder         : Lavc58.134.100 mjpeg
Side data:
cpb: bitrate max/min/avg: 0/0/200000 buffer size: 0 vbv_delay: N/A
frame=    1 fps=0.0 q=1.0 Lsize=N/A time=00:00:00.04 bitrate=N/A speed=3.65x    
video:100kB audio:0kB subtitle:0kB other streams:0kB global headers:0kB muxing overhead: unknown

real	0m0,048s
user	0m0,029s
sys	0m0,018s

$ time convert test.jpg -gravity South -pointsize 24 -fill white -annotate +0+10 "test.jpg" -quality 95 /tmp/out_im.jpg

real	0m0,046s
user	0m0,084s
sys	0m0,008s


