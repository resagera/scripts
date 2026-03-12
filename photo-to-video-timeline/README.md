
- `bash`-скрипт
`bash`-скрипт для генерации видео таймлайн из изображений, которые регистрирует сервис guard-service android app

Он делает так:
* берет папку стартовой даты, по умолчанию `вчера`
* берет файлы **с 06:00:00 стартового дня** или третьим параметром
* потом добавляет файлы из **следующей папки до 05:59:59**
* собирает `mp4` через `ffmpeg`
* создает `./cache` для временных файлов
* сохраняет итоговое видео в `./historic`
* считает размер выбранных jpg и размер итогового видео
* лог в файл
* выбор времени старта отдельным параметром
* в кадре рисуется имя файла без расширения

Параметры:

1. дата стартовой папки, по умолчанию `вчера`
2. fps, по умолчанию `10`
3. время старта, по умолчанию `06:00:00`

Как использовать:

```bash
chmod +x make_timeline.sh && ./make_timeline.sh
```

По умолчанию:

* дата = вчера
* fps = 10
* время старта `06:00:00`

Примеры:

```bash
./make_timeline.sh
./make_timeline.sh 2026-03-09
./make_timeline.sh 2026-03-09 12
./make_timeline.sh 2026-03-09 12 05:30:00
```

Что получится:

* временные файлы: `./cache/...`
* итоговое видео: `./historic/timeline_2026-03-10_10fps.mp4`

совместимую и с Linux, и с macOS


* поверх имени файла будет еще и дата/время из имени,
* будет прогресс-бар через `ffmpeg -progress`,
* и можно будет передать еще четвертый параметр: конечное время вместо “до следующего дня того же времени”.

- `go app`

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
