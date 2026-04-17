## Практическая работа №5. Вуйко Ярослава, ЭФМО-01-25
### Реализация HTTPS (TLS-сертификаты). Защита от SQL-инъекций. 17.04.2026

## Выбор варианта TLS

Для реализации HTTPS выбран вариант с TLS на NGINX. Это решение ближе к промышленным практикам, где SSL-сертификаты управляются на уровне reverse proxy, а само приложение остается "неосведомленным" о шифровании. Такой подход упрощает обслуживание сертификатов и позволяет централизованно управлять TLS для нескольких сервисов. Приложение tasks продолжает работать по HTTP на порту 8082 внутри Docker-сети, а NGINX принимает HTTPS-соединения на порту 8443 и проксирует их в приложение.

## Структура проекта

```
.
│   go.mod
│   go.sum
│   README.md
├── deploy/
│   └── monitoring/
│   │   ├── docker-compose.yml
│   │   └── init.sql
│   └── monitoring/
│   │   ├── docker-compose.yml
│   │   └── prometheus.yml
│   └── tls/
│       ├── cert.pem
│       ├── docker-compose.yml
│       ├── key.pem
│       └── nginx.conf
├───docs/
│     ├───  pz17_api.md
│     └───  pz17_diagram.md
├───proto/
│    └───auth.proto
├───pkg/
│    ├───auth.pb.go
│    └───auth_grpc.pb.go
├───services/
│   ├───auth/
│   │   ├───Dockerfile
│   │   ├───go.mod
│   │   ├───go.sum
│   │   ├───cmd/
│   │   │   └───auth/
│   │   │          └─── main.go
│   │   └───internal/
│   │       ├───grpc/
│   │       │       └─── server.go
│   │       ├───handler/
│   │       │       └─── auth_handler.go
│   │       └───service/
│   │               └─── auth_servise.go
│   └───tasks/
│       ├───Dockerfile
│       │   go.mod
│       │   go.sum
│       ├───cmd/
│       │   └───tasks/
│       │           └───main.go
│       └───internal/
│           ├───client/
│           │       └───auth_client.go
│           ├───handler/
│           │       └───task_handler.go
│           ├───metrics/
│           │       └─── metrics.go
│           ├───repository/
│           │       ├───postgres.go
│           │       └───task_repository.go
│           └───service/
│                   └───task_service.go
└───shared/
    ├───httpx/
    │       └───client.go
    ├───logger/
    │       └───logger.go
    ├───middleware/
    │       ├───accesslog.go
    │       ├───metrics.go
    │       └───requestid.go
    └───models/
            └───models.go
```




### Генерация сертификата

Был сгенерирован амоподписанный сертификат:

```
mkdir -p deploy/tls
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout deploy/tls/key.pem \
  -out deploy/tls/cert.pem \
  -days 365 \
  -subj "/CN=localhost"
```

Пояснение:
- `key.pem` — приватный ключ (добавлен в `.gitignore`)
- `cert.pem` — публичный сертификат

### Конфигурация NGINX

Файл `deploy/tls/nginx.conf` содержит следующую конфигурацию:

```

events {
    worker_connections 1024;
}

http {
    access_log /var/log/nginx/access.log;
    error_log /var/log/nginx/error.log;

    upstream tasks_backend {
        server tasks:8082;
    }

    server {
        listen 8443 ssl;
        server_name localhost;

        ssl_certificate     /etc/nginx/tls/cert.pem;
        ssl_certificate_key /etc/nginx/tls/key.pem;

        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers HIGH:!aNULL:!MD5;

        location / {
            proxy_pass http://tasks_backend;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto https;
            proxy_set_header X-Request-ID $http_x_request_id;
            proxy_set_header Authorization $http_authorization;
            
            proxy_connect_timeout 5s;
            proxy_read_timeout 10s;
        }
    }
}

```

Ключевые моменты: NGINX слушает 8443, использует самоподписанный сертификат и проксирует все запросы в контейнер tasks на порт 8082 с сохранением заголовков (Authorization, X-Request-ID).

### База данных и миграция

В качестве базы данных используется PostgreSQL. Таблица tasks:

```
CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    due_date VARCHAR(50),
    done BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Демонстрация SQL-инъекции

### Уязвимый запрос (было)

В исходной версии поиск задач формировался конкатенацией строк:

<img width="1267" height="227" alt="2026-04-16_20-00-17" src="https://github.com/user-attachments/assets/22ffb4f9-be1d-4505-930b-3133b4295dff" />

Этот код опасен, так как пользовательский ввод напрямую вставляется в SQL-запрос. Злоумышленник мог передать `' OR '1'='1` и получить все задачи системы.

Пример SQL инъекции при уязвимом запросе:

<img width="1365" height="837" alt="2026-04-16_19-53-43" src="https://github.com/user-attachments/assets/520c0e5a-cbeb-4d49-86c1-587dc681b849" />

В результате запроса пользователь получает все задачи из базы, хотя такого не должно быть

#### Безопасный запрос (стало)

Исправленная версия использует параметризованный запрос:

<img width="1043" height="372" alt="2026-04-16_20-06-20" src="https://github.com/user-attachments/assets/57c613a9-7c8a-43a8-9732-9472aed322d0" />

Теперь пользовательский ввод передается как параметр `$1`, а не вставляется в SQL. База данных воспринимает его как данные, а не как команду.

Пример SQL инъекции при исправленном безопасном запросе:

<img width="1382" height="468" alt="2026-04-16_20-05-34" src="https://github.com/user-attachments/assets/55915e2c-dd75-4b94-b50a-775e6d131e6f" />

В виде ответа пользователю выдается null поскольку БД воспринимает введенное значение как данные, а не команды. За счет этого не удается получить доступ к полному списку задач


### Инструкция запуска

```bash
# Переход в директорию с TLS-конфигурацией
cd deploy/tls

# Запуск всех сервисов (PostgreSQL, auth, tasks, nginx)
docker-compose up --build -d

# Просмотр логов
docker-compose logs -f
```


### Заключение

В результате работы были выполнены все поставленные задачи:
- Настроен HTTPS через NGINX с самоподписанным сертификатом
- Подключена PostgreSQL для хранения задач
- Устранена уязвимость SQL-инъекций через параметризованные запросы

### Контрольные вопросы
1. Какие свойства даёт TLS соединению?
TLS обеспечивает три основных свойства: шифрование данных, аутентификацию сервера и целостность данных.

2. Почему самоподписанный сертификат не подходит для реального продакшна?
Самоподписанный сертификат не заверен доверенным центром сертификации, поэтому браузеры и клиенты не могут проверить подлинность сервера. Это делает соединение уязвимым для атак "человек посередине", так как злоумышленник может подменить сертификат своим.

3. В чём отличие TLS-терминации на NGINX от TLS в приложении?
При TLS-терминации на NGINX сертификаты и шифрование управляются на уровне reverse proxy, а приложение работает по HTTP внутри сети. При TLS в приложении сертификаты загружаются и TLS-соединения обрабатываются самим приложением. Первый подход предпочтительнее в продакшене - проще управлять сертификатами для нескольких сервисов.

4. Как возникает SQL-инъекция?
SQL-инъекция возникает, когда пользовательский ввод напрямую подставляется в SQL-запрос через конкатенацию строк. Злоумышленник передает специально сформированные символы, которые изменяют логику запроса, заставляя БД выполнить unintended команды.

5. Почему параметризованный запрос защищает от SQLi?
Параметризованный запрос отделяет SQL-код от данных. Пользовательский ввод передается как параметр, а не вставляется в строку запроса. БД знает, что это данные, а не команды, поэтому любой вредоносный ввод обрабатывается как обычное значение, а не как SQL-инструкция.

6. Почему детали ошибок БД нельзя показывать клиенту?
Детали ошибок БД могут содержать чувствительную информацию: структуру таблиц, имена колонок, версию СУБД, фрагменты данных. Злоумышленник может использовать эту информацию для подготовки целенаправленных атак. Клиенту достаточно знать, что произошла внутренняя ошибка, а подробности логируются на сервере для администратора.
