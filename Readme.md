# Interplanet travel energy calculation: backend

Веб-сервис на Go (gin + GORM + PostgreSQL + MinIO). Все методы начинаются с `/api`.
Текущий пользователь зафиксирован (singleton `auth.CurrentUser()`, id = 1) до ЛР4.

## Таблицы

### users
| Поле | Тип | Описание |
|---|---|---|
| user_id | int, PK | идентификатор |
| username | text, unique, not null | логин |
| password | text, not null | bcrypt-хеш пароля |

### planet_pairs (услуги)
| Поле | Тип | Описание |
|---|---|---|
| planetpairs_id | int, PK | идентификатор |
| name | text | название карточки (задаётся при создании) |
| description | text | описание (заполняется при публикации) |
| status | text | `draft` / `published` / `deleted` |
| photo_url | text | имя файла изображения в MinIO |
| video_url | text | имя файла видео в MinIO |
| distance | int | расстояние (заполняется при публикации) |
| period | int | период сближения (заполняется при публикации) |
| creation_time | timestamp | дата создания |
| forming_time | timestamp | дата формирования (публикации) |
| creator_id | int, FK -> users | создатель |

### likes
| Поле | Тип | Описание |
|---|---|---|
| like_id | int, PK | идентификатор |
| planetpair_id | int, FK -> planet_pairs | услуга |
| user_id | int, FK -> users | пользователь |

Пара (planetpair_id, user_id) уникальна.

## HTTP-методы

### Услуги
| Метод | URL | Описание | Коды |
|---|---|---|---|
| GET | `/api/planet-pairs?range_min=&range_max=` | Список опубликованных с фильтром по расстоянию (оба параметра необязательны). В элементах есть `is_mine` (0/1, создатель услуги совпадает с текущим пользователем) | 200, 400 |
| GET | `/api/planet-pairs/feed` | Лента: все опубликованные по возрастанию id. В элементах есть `liked_by_me` (0/1) | 200 |
| GET | `/api/planet-pairs/feed?next=true&id={id}` | Одна карточка, следующая за `id`. После последней возвращается первая; без `id` возвращается первая. В ответе есть `liked_by_me` | 200, 400, 404 |
| GET | `/api/planet-pairs/draft` | Черновик текущего пользователя (не более одного), id не указывается | 200, 404 |
| POST | `/api/planet-pairs` | Создание черновика. `multipart/form-data`: `name`*, `photo` (jpg/png/gif/webp, до 5 МБ), `video` (mp4/webm, до 50 МБ). Остальные поля не принимаются. Файлы сохраняются в MinIO под сгенерированными латинскими именами, имена пишутся в БД | 201, 400, 409 (черновик уже есть) |
| PUT | `/api/planet-pairs/{id}/publish` | Заполнение остальных полей и публикация (draft -> published). JSON: `description`, `distance` > 0, `period` > 0 (все обязательны). Только создатель | 200, 400, 403, 404, 409 |
| DELETE | `/api/planet-pairs/{id}` | Мягкое удаление (status = deleted). Только услуги текущего пользователя. Ответ без тела | 204, 403, 404 |
| POST | `/api/planet-pairs/{id}/like` | JSON `{"like": 1}` ставит лайк, `{"like": 0}` снимает. Только для опубликованных. Ответ `{"like_count": N}` | 200, 400, 404 |

### Пользователи
| Метод | URL | Описание | Коды |
|---|---|---|---|
| POST | `/api/users` | Регистрация. JSON `{"username": "...", "password": "..."}` (username 3-50 символов, password 6-72). Ответ `{"id": ..., "username": "..."}` | 201, 400, 409 |
| POST | `/api/auth/login` | Аутентификация (заглушка, ЛР4). Ответ 200 без тела | 200 |
| POST | `/api/auth/logout` | Деавторизация (заглушка, ЛР4). Ответ 200 без тела | 200 |

## Поля услуги в ответах
| Поле | Где присутствует |
|---|---|
| id, name, description, status, photo_url, video_url, distance, period, creation_time, forming_time, creator_id, like_count | везде |
| is_mine | только в `GET /api/planet-pairs` |
| liked_by_me | только в `GET /api/planet-pairs/feed` |

`photo_url` и `video_url` в ответе это временные ссылки на файлы в MinIO (действуют 24 часа). В БД хранятся только имена файлов.

## Формат ответов
Успех: тело ответа это сам объект или массив, без обёртки `status` и `message`.
Ошибка: `{"status": "error", "description": "..."}` с соответствующим HTTP-кодом.
