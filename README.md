# telegram-bot-nats

Telegram Bot API ↔ NATS connector. Перенаправляет все входящие обновления Telegram в NATS и позволяет отправлять сообщения через NATS.

## Запуск

```bash
cp sample.env .env
# Отредактируйте .env — добавьте реальные токены ботов
go run .
```

## Переменные окружения

| Переменная   | Описание                                             | По умолчанию            |
| ------------ | ---------------------------------------------------- | ----------------------- |
| `NATS_URL`   | Адрес NATS-сервера                                   | `nats://localhost:4222` |
| `BOT_<NAME>` | Telegram Bot Token. `<NAME>` — произвольное имя бота | —                       |

Можно указать несколько ботов:

```env
BOT_MY_BOT=123456:AABBCC
BOT_SUPPORT=789012:XXYYZZ
```

## NATS Subjects

### Входящие (Telegram → NATS)

| Subject                       | Описание                    |
| ----------------------------- | --------------------------- |
| `telegram.<name>.in.update`   | Полный Update JSON          |
| `telegram.<name>.in.message`  | Новое сообщение             |
| `telegram.<name>.in.edited`   | Отредактированное сообщение |
| `telegram.<name>.in.callback` | Callback Query              |
| `telegram.<name>.in.inline`   | Inline Query                |

### Исходящие (NATS → Telegram)

Отправляйте JSON-запрос в соответствующий subject:

| Subject                                  | Telegram API метод  |
| ---------------------------------------- | ------------------- |
| `telegram.<name>.out.sendMessage`        | sendMessage         |
| `telegram.<name>.out.sendPhoto`          | sendPhoto           |
| `telegram.<name>.out.editMessage`        | editMessageText     |
| `telegram.<name>.out.deleteMessage`      | deleteMessage       |
| `telegram.<name>.out.answerCallback`     | answerCallbackQuery |
| `telegram.<name>.out.forwardMessage`     | forwardMessage      |
| `telegram.<name>.out.sendDocument`       | sendDocument        |
| `telegram.<name>.out.sendSticker`        | sendSticker         |
| `telegram.<name>.out.setMessageReaction` | setMessageReaction  |
| `telegram.<name>.out.raw`                | Произвольный метод  |

## Примеры использования

### Подписаться на все сообщения бота

```bash
nats sub "telegram.my_bot.in.message"
```

### Отправить сообщение

```bash
nats req "telegram.my_bot.out.sendMessage" '{"chat_id": 123456, "text": "Привет!"}'
```

### Ответить на callback

```bash
nats req "telegram.my_bot.out.answerCallback" '{"callback_query_id": "abc123", "text": "Done!"}'
```

### Произвольный API-вызов

```bash
nats req "telegram.my_bot.out.raw" '{"method": "getMe", "params": {}}'
```

При использовании `nats req` — ответ Telegram API вернётся как reply.

## Работа с медиа

### Приём медиа (Telegram → NATS)

Когда пользователь отправляет медиа боту, оно приходит в `telegram.<name>.in.message` как стандартный Telegram Message JSON. Файлы **не скачиваются** — приходит `file_id`, который можно использовать для скачивания или пересылки.

**Фото** — массив `photo` с разными размерами:

```json
{
  "chat": { "id": 123 },
  "photo": [
    { "file_id": "AgACAgIAA...", "width": 90, "height": 90 },
    { "file_id": "AgACAgIAB...", "width": 320, "height": 320 },
    { "file_id": "AgACAgIAC...", "width": 800, "height": 800 }
  ],
  "caption": "Подпись к фото"
}
```

**Документ**: поле `document` с `file_id`, `file_name`, `mime_type`
**Видео**: поле `video` с `file_id`, `duration`, `width`, `height`
**Аудио**: поле `audio` с `file_id`, `duration`, `title`, `performer`
**Голосовое**: поле `voice` с `file_id`, `duration`
**Стикер**: поле `sticker` с `file_id`, `emoji`
**Локация**: поле `location` с `latitude`, `longitude`
**Контакт**: поле `contact` с `phone_number`, `first_name`

### Отправка медиа (NATS → Telegram)

Медиа отправляется через JSON. Для файлов используйте `file_id` (пересылка уже загруженного) или URL.

> **Важно:** отправка бинарных файлов (multipart upload) не поддерживается — только `file_id` и URL.

#### Отправить фото

```bash
# По URL
nats req "telegram.my_bot.out.sendPhoto" '{
  "chat_id": 123,
  "photo": "https://example.com/image.jpg",
  "caption": "Описание"
}'

# По file_id (пересылка полученного фото)
nats req "telegram.my_bot.out.sendPhoto" '{
  "chat_id": 123,
  "photo": "AgACAgIAC..."
}'
```

#### Отправить документ

```bash
nats req "telegram.my_bot.out.sendDocument" '{
  "chat_id": 123,
  "document": "https://example.com/report.pdf",
  "caption": "Отчёт"
}'
```

#### Отправить видео

```bash
nats req "telegram.my_bot.out.raw" '{
  "method": "sendVideo",
  "params": {
    "chat_id": 123,
    "video": "https://example.com/video.mp4",
    "caption": "Видео"
  }
}'
```

#### Отправить аудио

```bash
nats req "telegram.my_bot.out.raw" '{
  "method": "sendAudio",
  "params": {
    "chat_id": 123,
    "audio": "https://example.com/song.mp3",
    "title": "Название"
  }
}'
```

#### Отправить голосовое сообщение

```bash
nats req "telegram.my_bot.out.raw" '{
  "method": "sendVoice",
  "params": {
    "chat_id": 123,
    "voice": "https://example.com/voice.ogg"
  }
}'
```

#### Отправить локацию

```bash
nats req "telegram.my_bot.out.raw" '{
  "method": "sendLocation",
  "params": {
    "chat_id": 123,
    "latitude": 55.7558,
    "longitude": 37.6173
  }
}'
```

#### Отправить контакт

```bash
nats req "telegram.my_bot.out.raw" '{
  "method": "sendContact",
  "params": {
    "chat_id": 123,
    "phone_number": "+79001234567",
    "first_name": "Иван"
  }
}'
```

#### Скачать файл по file_id

```bash
# 1. Получить путь к файлу
nats req "telegram.my_bot.out.raw" '{
  "method": "getFile",
  "params": {"file_id": "AgACAgIAC..."}
}'
# Ответ содержит file_path

# 2. Скачать: https://api.telegram.org/file/bot<TOKEN>/<file_path>
```

### Типичный сценарий: получить фото и переслать

```
1. Пользователь шлёт фото боту
2. В NATS приходит telegram.my_bot.in.message с photo[].file_id
3. Ваш сервис берёт file_id последнего (самого большого) фото
4. Отправляет в другой чат:
   nats req "telegram.my_bot.out.sendPhoto" {"chat_id": 456, "photo": "<file_id>"}
```
