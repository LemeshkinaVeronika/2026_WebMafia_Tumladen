# Kubernetes

Манифесты разворачивают два workload из одного образа:

- `tumladan-api` — две реплики HTTP/WebSocket API за `ClusterIP Service`;
- `tumladan-event-worker` — публикация Transactional Outbox и Kafka consumer;
- `tumladan-migrate` — одноразовый Job для PostgreSQL-миграций.

PostgreSQL, Redis, Kafka и MinIO должны быть доступны в кластере. Их адреса по
умолчанию заданы в `configmap.yaml` и могут быть заменены через overlay.

## Запуск

Сначала создайте Secret. Не применяйте пример с `REPLACE_WITH_*` в production:

```bash
kubectl apply -f k8s/namespace.yaml
cp k8s/secret.example.yaml /tmp/tumladan-secret.yaml
# Заполните /tmp/tumladan-secret.yaml безопасными значениями.
kubectl apply -f /tmp/tumladan-secret.yaml
```

Примените конфигурацию и дождитесь миграции перед rollout приложения:

```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/migration-job.yaml
kubectl wait --for=condition=complete job/tumladan-migrate -n tumladan --timeout=120s
kubectl apply -k k8s
kubectl rollout status deployment/tumladan-api -n tumladan
kubectl rollout status deployment/tumladan-event-worker -n tumladan
```

Для конкретного immutable-тега образа:

```bash
cd k8s
kustomize edit set image ghcr.io/lemeshkinaveronika/2026_webmafia_tumladan=ghcr.io/lemeshkinaveronika/2026_webmafia_tumladan:sha-<commit>
kubectl apply -k .
```

Публичный Ingress намеренно не включён в base: домен, TLS issuer и ingress class
зависят от целевого кластера.
