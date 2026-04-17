# EGN-Proxy-Core

Небольшое ядро L7 reverse proxy на Go: балансировка, health-check и базовая защита от перегрузки/флуда.

## Что есть сейчас

- weighted round-robin по backend-ам;
- маршрутизация по `Host` (можно задать `*`);
- активные health-check запросы;
- rate limiting: global + per-IP;
- временный бан IP при постоянных превышениях лимита;
- ограничение `max_inflight` для защиты от перегрева;
- сервисные endpoint-ы: `/live`, `/ready`, `/metrics`.

## Запуск

```bash
go run ./cmd/egn-proxy-core
```

С конфигом:

```bash
go run ./cmd/egn-proxy-core -config ./config.json
```

## Пример конфига

```json
{
  "listen_addr": ":8080",
  "read_timeout": 5000000000,
  "write_timeout": 15000000000,
  "idle_timeout": 60000000000,
  "max_header_bytes": 1048576,
  "max_inflight": 50000,
  "trusted_proxy_header": "X-Forwarded-For",
  "backends": [
    {
      "name": "frontend-a",
      "url": "http://10.0.1.11:8080",
      "weight": 4,
      "hostnames": ["example.com", "www.example.com"]
    },
    {
      "name": "frontend-b",
      "url": "http://10.0.1.12:8080",
      "weight": 2,
      "hostnames": ["*"]
    }
  ],
  "ddos": {
    "global_rps_limit": 75000,
    "per_ip_rps_limit": 200,
    "per_ip_burst": 500,
    "global_burst": 12000,
    "ban_threshold_per_minute": 6000,
    "ban_duration": 900000000000,
    "cleanup_interval": 30000000000
  },
  "health": {
    "check_interval": 2000000000,
    "path": "/health",
    "timeout": 1200000000
  }
}
```

> `time.Duration` в JSON задаётся в наносекундах.

## Что важно понимать

Этот репозиторий — ядро сервиса. Для стабильных 50k+ RPS под реальными атаками обычно нужен ещё внешний слой:

- L4 балансировка перед нодами прокси;
- корректный kernel/network tuning;
- отдельный TLS-слой (если трафик тяжёлый);
- нагрузочные тесты на вашем профиле запросов.

## Проверка

```bash
go test ./...
```
