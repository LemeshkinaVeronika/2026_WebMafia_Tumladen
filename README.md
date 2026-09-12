# Tumladen (Backend)

Tumladen - онлайн-платформа для игры в настольные игры.

Backend построен на Go, PostgreSQL, Kafka, Redis, MinIO и Centrifuge. Redis используется
для одноразовых WebSocket-тикетов, межрепличной доставки realtime-событий,
distributed presence и сериализации игровых действий на уровне комнаты.
Результат матча сохраняется вместе с событием `match.finished.v1` по паттерну
Transactional Outbox. Отдельный worker публикует события в Kafka, а идемпотентный
consumer асинхронно обновляет статистику игроков и разблокирует достижения.

## Команда проекта

- [Артем Голубев](https://github.com/Xyzdat)
- [Дмитрий Дорофеев](https://github.com/poopaapopa)
- [Вероника Лемешкина](https://github.com/LemeshkinaVeronika)

## Связанные проекты

- **Frontend-часть:** [Tumladen Frontend](https://github.com/poopaapopa/Tumladen-Frontend)

## Запуск проекта

Для локального запуска необходимы Docker, Docker Compose и `make`. 

```bash
git clone https://github.com/LemeshkinaVeronika/2026_WebMafia_Tumladan.git
cd 2026_WebMafia_Tumladan

docker network inspect tumladan_net >/dev/null 2>&1 || docker network create tumladan_net

make docker-build
make docker-up
```

Состояние контейнеров и логи можно проверить командами:

```bash
docker compose ps
make docker-logs
```

MinIO API доступен на `localhost:9000`, web-консоль — на
`http://localhost:9001`, Kafka — на `localhost:9092`.

Для остановки или удаления контейнеров локального окружения:

```bash
make docker-stop
make docker-down
```

## Реализация
- [Ссылка на deploy](http://87.239.104.134:8080/)
