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
| planet_start | text | планета отправления |
| planet_end | text | планета назначения |
| description | text | описание |
| status | text | `draft` / `published` / `deleted` |
| photo_url | text | имя файла изображения в MinIO |
| video_url | text | имя файла видео в MinIO |
| distance | int | расстояние |
| period | int | период сближения |
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

## Статусы услуги
`draft -> published`, `draft -> deleted`, `published -> deleted`.
Вернуть в `draft` нельзя. Записи `deleted` клиенту не передаются.
Системные поля (id, status, creator_id, даты) клиентом не задаются.

## HTTP-методы

### Услуги
| Метод | URL | Описание | Коды |
|---|---|---|---|
| GET | `/api/planet-pairs?range_min=&range_max=` | Список опубликованных с фильтром по расстоянию. В каждом элементе `is_mine` (0/1), `like_count`, `liked_by_me` | 200, 400 |
| GET | `/api/planet-pairs/feed?limit=&offset=` | Лента опубликованных (новые сверху, limit 1..50, по умолчанию 10) | 200, 400 |
| GET | `/api/planet-pairs/draft` | Черновик текущего пользователя (не более одного) | 200, 404 |
| POST | `/api/planet-pairs` | Создание черновика. `multipart/form-data`: `planet_start`*, `planet_end`*, `description`, `distance`, `period`, файлы `photo` (jpg/png/gif/webp, до 5 МБ), `video` (mp4/webm, до 50 МБ). Файлы сохраняются в MinIO под латинскими именами, имена пишутся в БД | 201, 400, 409 (черновик уже есть) |
| PUT | `/api/planet-pairs/{id}/publish` | Публикация (draft -> published). Нужны description, distance > 0, period > 0. Только создатель | 200, 400, 403, 404, 409 |
| DELETE | `/api/planet-pairs/{id}` | Мягкое удаление (status = deleted). Только услуги текущего пользователя | 200, 403, 404 |
| POST | `/api/planet-pairs/{id}/like` | Тело JSON `{"like": 1}` ставит лайк, `{"like": 0}` снимает. Только для опубликованных | 200, 400, 404 |

### Пользователи
| Метод | URL | Описание | Коды |
|---|---|---|---|
| POST | `/api/users` | Регистрация. JSON `{"username": "...", "password": "..."}` | 201, 400, 409 |
| POST | `/api/auth/login` | Аутентификация (заглушка, ЛР4) | 200 |
| POST | `/api/auth/logout` | Деавторизация (заглушка, ЛР4) | 200 |

## Формат ответов
Успех: `{"status": "success", "data": ...}`.
Ошибка: `{"status": "error", "description": "..."}` с соответствующим HTTP-кодом.
