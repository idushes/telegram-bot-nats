# telegram-bot-nats

Telegram Bot API ↔ NATS connector (webhook mode). Принимает обновления от Telegram через HTTP webhook и публикует их в NATS. Позволяет отправлять сообщения через NATS.

## Запуск

```bash
cp sample.env .env
# Отредактируйте .env — добавьте реальные токены ботов
go run .
```

## Переменные окружения

| Переменная         | Описание                                             | По умолчанию            |
| ------------------ | ---------------------------------------------------- | ----------------------- |
| `NATS_URL`         | Адрес NATS-сервера                                   | `nats://localhost:4222` |
| `BOT_<NAME>`       | Telegram Bot Token. `<NAME>` — произвольное имя бота | —                       |
| `WEBHOOK_BASE_URL` | Публичный URL сервера (обязательная)                 | —                       |
| `PORT`             | Порт HTTP-сервера                                    | `8080`                  |
| `WEBHOOK_SECRET`   | Secret token для верификации запросов от Telegram    | —                       |

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

## Быстрый старт

```bash
# Подписаться на все входящие сообщения
nats sub "telegram.my_bot.in.>"

# Отправить текстовое сообщение
nats req "telegram.my_bot.out.sendMessage" '{"chat_id": 123, "text": "Привет!"}'

# Произвольный API-вызов
nats req "telegram.my_bot.out.raw" '{"method": "getMe", "params": {}}'
```

При использовании `nats req` — ответ Telegram API вернётся как reply.

---

## Справочник: входящие сообщения (Telegram → NATS)

Все примеры ниже — JSON, который приходит в `telegram.<name>.in.message` (или соответствующий `in.*` subject). Формат полностью совпадает с [Telegram Bot API](https://core.telegram.org/bots/api#message).

> **Примечание:** поля `message_id`, `from`, `chat`, `date` присутствуют во всех сообщениях. Для краткости некоторые показаны один раз.

### Текстовое сообщение

```json
{
  "message_id": 100,
  "from": {
    "id": 111222333,
    "is_bot": false,
    "first_name": "Иван",
    "last_name": "Петров",
    "username": "ivan_p",
    "language_code": "ru"
  },
  "chat": {
    "id": 111222333,
    "first_name": "Иван",
    "last_name": "Петров",
    "username": "ivan_p",
    "type": "private"
  },
  "date": 1708300000,
  "text": "Привет, бот!"
}
```

### Текст с entities (команда, ссылка, mention)

```json
{
  "message_id": 101,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300001,
  "text": "/start hello @other_bot https://example.com",
  "entities": [
    { "offset": 0, "length": 6, "type": "bot_command" },
    { "offset": 13, "length": 10, "type": "mention" },
    { "offset": 24, "length": 19, "type": "url" }
  ]
}
```

### Ответ на сообщение (reply)

```json
{
  "message_id": 102,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300002,
  "text": "Это ответ",
  "reply_to_message": {
    "message_id": 100,
    "from": { "id": 999888777, "is_bot": true, "first_name": "MyBot" },
    "chat": { "id": 111222333, "type": "private" },
    "date": 1708299000,
    "text": "Исходное сообщение"
  }
}
```

### Пересланное сообщение (forward)

```json
{
  "message_id": 103,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300003,
  "forward_origin": {
    "type": "user",
    "sender_user": { "id": 444555666, "first_name": "Анна" },
    "date": 1708200000
  },
  "text": "Пересланный текст"
}
```

### Фото

```json
{
  "message_id": 104,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300004,
  "photo": [
    {
      "file_id": "AgACAgIAAx0CX...sm",
      "file_unique_id": "AQADAgAT...sm",
      "file_size": 1234,
      "width": 90,
      "height": 90
    },
    {
      "file_id": "AgACAgIAAx0CX...md",
      "file_unique_id": "AQADAgAT...md",
      "file_size": 12345,
      "width": 320,
      "height": 320
    },
    {
      "file_id": "AgACAgIAAx0CX...lg",
      "file_unique_id": "AQADAgAT...lg",
      "file_size": 54321,
      "width": 800,
      "height": 800
    }
  ],
  "caption": "Подпись к фото",
  "caption_entities": [{ "offset": 0, "length": 14, "type": "bold" }]
}
```

> Массив `photo` содержит одно фото в нескольких размерах. Последний элемент — максимальное качество.

### Документ (файл)

```json
{
  "message_id": 105,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300005,
  "document": {
    "file_id": "BQACAgIAAx0CX...",
    "file_unique_id": "AgADXgQ...",
    "file_name": "report.pdf",
    "mime_type": "application/pdf",
    "file_size": 204800
  },
  "caption": "Отчёт за месяц"
}
```

### Видео

```json
{
  "message_id": 106,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300006,
  "video": {
    "file_id": "BAACAgIAAx0CX...",
    "file_unique_id": "AgADbAQ...",
    "width": 1920,
    "height": 1080,
    "duration": 30,
    "mime_type": "video/mp4",
    "file_size": 5242880
  },
  "caption": "Видео"
}
```

### Аудио (музыка)

```json
{
  "message_id": 107,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300007,
  "audio": {
    "file_id": "CQACAgIAAx0CX...",
    "file_unique_id": "AgADcAQ...",
    "duration": 210,
    "performer": "Исполнитель",
    "title": "Название трека",
    "mime_type": "audio/mpeg",
    "file_size": 3145728
  }
}
```

### Голосовое сообщение (voice)

```json
{
  "message_id": 108,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300008,
  "voice": {
    "file_id": "AwACAgIAAx0CX...",
    "file_unique_id": "AgADeAQ...",
    "duration": 5,
    "mime_type": "audio/ogg",
    "file_size": 12345
  }
}
```

### Видеосообщение (кружочек, video_note)

```json
{
  "message_id": 109,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300009,
  "video_note": {
    "file_id": "DQACAgIAAx0CX...",
    "file_unique_id": "AgADfAQ...",
    "length": 240,
    "duration": 10,
    "file_size": 102400
  }
}
```

### Стикер

```json
{
  "message_id": 110,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300010,
  "sticker": {
    "file_id": "CAACAgIAAx0CX...",
    "file_unique_id": "AgADjAQ...",
    "type": "regular",
    "width": 512,
    "height": 512,
    "is_animated": false,
    "is_video": false,
    "emoji": "😀",
    "set_name": "MyStickerPack"
  }
}
```

### Локация

```json
{
  "message_id": 111,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300011,
  "location": {
    "latitude": 55.755811,
    "longitude": 37.617617
  }
}
```

### Venue (место)

```json
{
  "message_id": 112,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300012,
  "venue": {
    "location": { "latitude": 55.755811, "longitude": 37.617617 },
    "title": "Красная площадь",
    "address": "Красная площадь, Москва"
  }
}
```

### Контакт

```json
{
  "message_id": 113,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300013,
  "contact": {
    "phone_number": "+79001234567",
    "first_name": "Анна",
    "last_name": "Смирнова",
    "user_id": 444555666
  }
}
```

### Опрос (poll)

```json
{
  "message_id": 114,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300014,
  "poll": {
    "id": "5765...",
    "question": "Какой язык лучше?",
    "options": [
      { "text": "Go", "voter_count": 0 },
      { "text": "Rust", "voter_count": 0 }
    ],
    "total_voter_count": 0,
    "is_closed": false,
    "is_anonymous": true,
    "type": "regular",
    "allows_multiple_answers": false
  }
}
```

### Кубик / игра (dice)

```json
{
  "message_id": 115,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708300015,
  "dice": {
    "emoji": "🎲",
    "value": 4
  }
}
```

### Новый участник группы

```json
{
  "message_id": 116,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": -1001234567890, "type": "supergroup", "title": "My Group" },
  "date": 1708300016,
  "new_chat_members": [
    { "id": 444555666, "first_name": "Анна", "is_bot": false }
  ]
}
```

### Выход участника из группы

```json
{
  "message_id": 117,
  "from": { "id": 444555666, "first_name": "Анна" },
  "chat": { "id": -1001234567890, "type": "supergroup", "title": "My Group" },
  "date": 1708300017,
  "left_chat_member": { "id": 444555666, "first_name": "Анна", "is_bot": false }
}
```

### Закрепление сообщения

```json
{
  "message_id": 118,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": -1001234567890, "type": "supergroup", "title": "My Group" },
  "date": 1708300018,
  "pinned_message": {
    "message_id": 100,
    "from": { "id": 111222333, "first_name": "Иван" },
    "chat": { "id": -1001234567890, "type": "supergroup" },
    "date": 1708200000,
    "text": "Важное сообщение"
  }
}
```

### Сообщение в группе / супергруппе

```json
{
  "message_id": 119,
  "from": { "id": 111222333, "first_name": "Иван", "username": "ivan_p" },
  "chat": {
    "id": -1001234567890,
    "title": "My Group",
    "type": "supergroup"
  },
  "date": 1708300019,
  "text": "Привет группе!"
}
```

### Callback Query (`in.callback`)

```json
{
  "id": "1234567890",
  "from": { "id": 111222333, "first_name": "Иван", "username": "ivan_p" },
  "message": {
    "message_id": 100,
    "from": { "id": 999888777, "is_bot": true, "first_name": "MyBot" },
    "chat": { "id": 111222333, "type": "private" },
    "date": 1708299000,
    "text": "Выберите вариант:",
    "reply_markup": {
      "inline_keyboard": [
        [
          { "text": "Вариант A", "callback_data": "option_a" },
          { "text": "Вариант B", "callback_data": "option_b" }
        ]
      ]
    }
  },
  "chat_instance": "123456",
  "data": "option_a"
}
```

### Inline Query (`in.inline`)

```json
{
  "id": "9876543210",
  "from": { "id": 111222333, "first_name": "Иван", "username": "ivan_p" },
  "query": "поиск чего-то",
  "offset": ""
}
```

### Отредактированное сообщение (`in.edited`)

```json
{
  "message_id": 100,
  "from": { "id": 111222333, "first_name": "Иван" },
  "chat": { "id": 111222333, "type": "private" },
  "date": 1708299000,
  "edit_date": 1708300020,
  "text": "Отредактированный текст"
}
```

---

## Справочник: исходящие запросы (NATS → Telegram)

Все примеры — JSON-payload, который нужно отправить в соответствующий `telegram.<name>.out.*` subject. Формат совпадает с [Telegram Bot API](https://core.telegram.org/bots/api#available-methods).

> **Файлы (фото, видео, документы):** передавайте `file_id` (из полученного ранее сообщения) или URL. Бинарный multipart upload не поддерживается.

### sendMessage — отправить текстовое сообщение

**Subject:** `telegram.<name>.out.sendMessage`

Простой текст:

```json
{ "chat_id": 123, "text": "Привет!" }
```

С Markdown-разметкой:

```json
{
  "chat_id": 123,
  "text": "*Жирный* _курсив_ `код` [ссылка](https://example.com)",
  "parse_mode": "MarkdownV2"
}
```

С HTML-разметкой:

```json
{
  "chat_id": 123,
  "text": "<b>Жирный</b> <i>курсив</i> <code>код</code> <a href='https://example.com'>ссылка</a>",
  "parse_mode": "HTML"
}
```

Ответ на конкретное сообщение (reply):

```json
{
  "chat_id": 123,
  "text": "Это ответ",
  "reply_parameters": { "message_id": 100 }
}
```

Без превью ссылок:

```json
{
  "chat_id": 123,
  "text": "Ссылка https://example.com без превью",
  "link_preview_options": { "is_disabled": true }
}
```

С inline-клавиатурой:

```json
{
  "chat_id": 123,
  "text": "Выберите вариант:",
  "reply_markup": {
    "inline_keyboard": [
      [
        { "text": "✅ Да", "callback_data": "yes" },
        { "text": "❌ Нет", "callback_data": "no" }
      ],
      [{ "text": "🌐 Открыть сайт", "url": "https://example.com" }]
    ]
  }
}
```

С обычной клавиатурой (reply keyboard):

```json
{
  "chat_id": 123,
  "text": "Выберите:",
  "reply_markup": {
    "keyboard": [
      [{ "text": "📊 Статистика" }, { "text": "⚙️ Настройки" }],
      [{ "text": "📍 Отправить локацию", "request_location": true }]
    ],
    "resize_keyboard": true,
    "one_time_keyboard": true
  }
}
```

Убрать клавиатуру:

```json
{
  "chat_id": 123,
  "text": "Клавиатура убрана",
  "reply_markup": { "remove_keyboard": true }
}
```

### sendPhoto — отправить фото

**Subject:** `telegram.<name>.out.sendPhoto`

```json
{
  "chat_id": 123,
  "photo": "https://example.com/image.jpg",
  "caption": "<b>Подпись</b> к фото",
  "parse_mode": "HTML"
}
```

По file_id (пересылка ранее полученного):

```json
{ "chat_id": 123, "photo": "AgACAgIAAx0CX...lg" }
```

### sendDocument — отправить документ

**Subject:** `telegram.<name>.out.sendDocument`

```json
{
  "chat_id": 123,
  "document": "https://example.com/report.pdf",
  "caption": "Отчёт за февраль"
}
```

### sendVideo — отправить видео

**Subject:** `telegram.<name>.out.sendVideo`

```json
{
  "chat_id": 123,
  "video": "https://example.com/video.mp4",
  "caption": "Видео",
  "supports_streaming": true
}
```

### sendAudio — отправить аудио

**Subject:** `telegram.<name>.out.sendAudio`

```json
{
  "chat_id": 123,
  "audio": "https://example.com/song.mp3",
  "title": "Название",
  "performer": "Исполнитель"
}
```

### sendVoice — отправить голосовое

**Subject:** `telegram.<name>.out.sendVoice`

```json
{
  "chat_id": 123,
  "voice": "AwACAgIAAx0CX..."
}
```

### sendVideoNote — отправить кружочек

**Subject:** `telegram.<name>.out.sendVideoNote`

```json
{
  "chat_id": 123,
  "video_note": "DQACAgIAAx0CX..."
}
```

### sendSticker — отправить стикер

**Subject:** `telegram.<name>.out.sendSticker`

```json
{ "chat_id": 123, "sticker": "CAACAgIAAx0CX..." }
```

### sendLocation — отправить локацию

**Subject:** `telegram.<name>.out.sendLocation`

```json
{
  "chat_id": 123,
  "latitude": 55.755811,
  "longitude": 37.617617
}
```

### sendVenue — отправить место

**Subject:** `telegram.<name>.out.sendVenue`

```json
{
  "chat_id": 123,
  "latitude": 55.755811,
  "longitude": 37.617617,
  "title": "Красная площадь",
  "address": "Красная площадь, Москва"
}
```

### sendContact — отправить контакт

**Subject:** `telegram.<name>.out.sendContact`

```json
{
  "chat_id": 123,
  "phone_number": "+79001234567",
  "first_name": "Анна",
  "last_name": "Смирнова"
}
```

### sendPoll — отправить опрос

**Subject:** `telegram.<name>.out.sendPoll`

```json
{
  "chat_id": 123,
  "question": "Какой язык лучше?",
  "options": [{ "text": "Go" }, { "text": "Rust" }, { "text": "Python" }],
  "is_anonymous": false
}
```

### sendDice — отправить кубик/игру

**Subject:** `telegram.<name>.out.sendDice`

```json
{ "chat_id": 123, "emoji": "🎲" }
```

Возможные emoji: `🎲` `🎯` `🏀` `⚽` `🎳` `🎰`

### forwardMessage — переслать сообщение

**Subject:** `telegram.<name>.out.forwardMessage`

```json
{
  "chat_id": 456,
  "from_chat_id": 123,
  "message_id": 100
}
```

### copyMessage — скопировать сообщение (без пометки "переслано")

**Subject:** `telegram.<name>.out.copyMessage`

```json
{
  "chat_id": 456,
  "from_chat_id": 123,
  "message_id": 100,
  "caption": "Новая подпись"
}
```

### editMessageText — редактировать текст сообщения

**Subject:** `telegram.<name>.out.editMessageText`

```json
{
  "chat_id": 123,
  "message_id": 100,
  "text": "Обновлённый текст",
  "parse_mode": "HTML"
}
```

С обновлением inline-клавиатуры:

```json
{
  "chat_id": 123,
  "message_id": 100,
  "text": "Вы выбрали: Да ✅",
  "reply_markup": {
    "inline_keyboard": [[{ "text": "↩️ Отменить", "callback_data": "undo" }]]
  }
}
```

### editMessageReplyMarkup — обновить только клавиатуру

**Subject:** `telegram.<name>.out.editMessageReplyMarkup`

```json
{
  "chat_id": 123,
  "message_id": 100,
  "reply_markup": {
    "inline_keyboard": [[{ "text": "Новая кнопка", "callback_data": "new" }]]
  }
}
```

### deleteMessage — удалить сообщение

**Subject:** `telegram.<name>.out.deleteMessage`

```json
{ "chat_id": 123, "message_id": 100 }
```

### answerCallbackQuery — ответить на нажатие inline-кнопки

**Subject:** `telegram.<name>.out.answerCallbackQuery`

Тихий ответ (убрать часики):

```json
{ "callback_query_id": "1234567890" }
```

С уведомлением:

```json
{
  "callback_query_id": "1234567890",
  "text": "Готово! ✅"
}
```

С alert-окном:

```json
{
  "callback_query_id": "1234567890",
  "text": "⚠️ Вы уверены?",
  "show_alert": true
}
```

### answerInlineQuery — ответить на inline-запрос

**Subject:** `telegram.<name>.out.answerInlineQuery`

```json
{
  "inline_query_id": "9876543210",
  "results": [
    {
      "type": "article",
      "id": "1",
      "title": "Результат 1",
      "description": "Описание результата",
      "input_message_content": { "message_text": "Текст сообщения" }
    }
  ],
  "cache_time": 10
}
```

### setMessageReaction — поставить реакцию

**Subject:** `telegram.<name>.out.setMessageReaction`

```json
{
  "chat_id": 123,
  "message_id": 100,
  "reaction": [{ "type": "emoji", "emoji": "👍" }]
}
```

### pinChatMessage — закрепить сообщение

**Subject:** `telegram.<name>.out.pinChatMessage`

```json
{
  "chat_id": 123,
  "message_id": 100,
  "disable_notification": true
}
```

### getChatMember — информация о участнике чата

**Subject:** `telegram.<name>.out.getChatMember`

```json
{ "chat_id": -1001234567890, "user_id": 111222333 }
```

### getFile — получить путь для скачивания файла

**Subject:** `telegram.<name>.out.getFile`

```json
{ "file_id": "AgACAgIAAx0CX...lg" }
```

Ответ содержит `file_path`. Скачать: `https://api.telegram.org/file/bot<TOKEN>/<file_path>`

### raw — произвольный API-метод

**Subject:** `telegram.<name>.out.raw`

Для любого метода, не имеющего отдельного subject:

```json
{
  "method": "banChatMember",
  "params": {
    "chat_id": -1001234567890,
    "user_id": 444555666,
    "until_date": 1708400000
  }
}
```

---

## Типичные сценарии

### Эхо-бот

```
1. Подписаться на telegram.my_bot.in.message
2. Получить JSON с полем "text"
3. Отправить в telegram.my_bot.out.sendMessage:
   {"chat_id": <chat.id из входящего>, "text": <text из входящего>}
```

### Бот с inline-кнопками

```
1. Отправить сообщение с клавиатурой через out.sendMessage (с reply_markup)
2. Подписаться на telegram.my_bot.in.callback
3. При нажатии кнопки — получить callback_query с data
4. Ответить через out.answerCallbackQuery (убрать часики)
5. Обновить сообщение через out.editMessageText
```

### Пересылка фото между чатами

```
1. Получить сообщение с photo в telegram.my_bot.in.message
2. Взять file_id последнего элемента массива photo (макс. размер)
3. Отправить в telegram.my_bot.out.sendPhoto:
   {"chat_id": <другой чат>, "photo": "<file_id>"}
```
