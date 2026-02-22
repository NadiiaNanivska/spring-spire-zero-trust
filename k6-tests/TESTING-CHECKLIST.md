# 📋 JWT vs SPIRE Testing Checklist
## Повний гайд для проведення експериментів

---

## 🎯 Загальна мета
Порівняти накладні витрати JWT та SPIRE для service-to-service автентифікації через 5 сценаріїв тестування.

---

## 📅 Розклад тестування (5 днів)

### День 1: Підготовка
- [ ] Встановити k6: `brew install k6` або https://k6.io/docs/get-started/installation/
- [ ] Перевірити доступність обох стендів
- [ ] Налаштувати Grafana dashboards
- [ ] Отримати JWT токен (якщо потрібно)
- [ ] Зробити baseline screenshots (idle state)

### День 2: Scenarios 1-2
- [ ] Запустити Scenario 1 для JWT (20 хв)
- [ ] Експортувати метрики з Grafana
- [ ] Пауза 30 хв
- [ ] Запустити Scenario 1 для SPIRE (20 хв)
- [ ] Експортувати метрики
- [ ] Пауза 1 год
- [ ] Повторити для Scenario 2

### День 3: Scenarios 3-4
- [ ] Запустити Scenario 3 для обох стендів
- [ ] Запустити Scenario 4 для обох стендів
- [ ] Зберегти всі результати

### День 4: Scenario 5 + повтори
- [ ] Запустити Scenario 5 (з kubectl scale)
- [ ] Повторити критичні тести для валідації

### День 5: Аналіз
- [ ] Агрегувати результати
- [ ] Створити порівняльні графіки
- [ ] Написати висновки

---

## 🔧 Попередні налаштування

### 1. Перевірка endpoints

```bash
# JWT стенд
curl http://localhost:8080/actuator/health
curl http://localhost:8080/actuator/prometheus

# SPIRE стенд
curl http://localhost:8081/actuator/health
curl http://localhost:8081/actuator/prometheus

# Prometheus
curl http://localhost:9090/-/healthy

# Grafana
curl http://localhost:3000/api/health
```

### 2. Отримання JWT токена

```bash
# Залежить від вашої імплементації
# Приклад:
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"test"}' \
  | jq -r '.token'

# Збережіть токен у змінну
export JWT_TOKEN="eyJhbGc..."
```

### 3. Налаштування конфігурації

```bash
# Створіть файл config.env
cat > config.env <<EOF
JWT_URL=http://localhost:8080
SPIRE_URL=http://localhost:8081
JWT_TOKEN=your-jwt-token-here
PROMETHEUS_URL=http://localhost:9090
GRAFANA_URL=http://localhost:3000
EOF

# Завантажте конфігурацію
source config.env
```

---

## 🧪 Детальні інструкції по сценаріям

### Scenario 1: Steady State Baseline
**Час виконання:** 20 хв × 2 стенди = 40 хв

#### Перед запуском:
1. Відкрийте Grafana dashboard
2. Переконайтеся що обидва стенди в idle state (RPS ≈ 0)
3. Зробіть screenshot початкового стану

#### Команди:
```bash
# JWT
k6 run -e BASE_URL=$JWT_URL -e AUTH_TYPE=JWT -e JWT_TOKEN=$JWT_TOKEN \
  scenario1-steady-state.js

# Пауза 30 хв для cooling down
sleep 30m

# SPIRE
k6 run -e BASE_URL=$SPIRE_URL -e AUTH_TYPE=SPIRE \
  scenario1-steady-state.js
```

#### Що спостерігати в Grafana:
- [ ] Average Latency залишається стабільним
- [ ] CPU Usage не зростає лінійно
- [ ] Memory stable (no leaks)
- [ ] RPS = 100 ± 5

#### Метрики для збереження:
- [ ] Screenshot "Performance & Latency" панелі
- [ ] Screenshot "Resource Utilization" панелі
- [ ] Export CSV: p99 latency, avg latency, CPU, memory
- [ ] Зберегти `results-scenario1-JWT.json` та `results-scenario1-SPIRE.json`

---

### Scenario 2: Connection Churn
**Час виконання:** 10 хв × 2 стенди = 20 хв

#### Особливість:
⚠️ **Keep-Alive вимкнено!** Кожен запит = нове TCP з'єднання

#### Команди:
```bash
# JWT
k6 run -e BASE_URL=$JWT_URL -e AUTH_TYPE=JWT -e JWT_TOKEN=$JWT_TOKEN \
  scenario2-connection-churn.js

sleep 30m

# SPIRE
k6 run -e BASE_URL=$SPIRE_URL -e AUTH_TYPE=SPIRE \
  scenario2-connection-churn.js
```

#### Що спостерігати в Grafana:
- [ ] **SPIRE має показати вищий latency** через mTLS handshake
- [ ] Network Traffic - сплески на кожен запит (сертифікати)
- [ ] "Bytes per Request" значно більший у SPIRE
- [ ] Connection time (http_req_connecting) вищий у SPIRE

#### Очікувані результати:
| Метрика | JWT | SPIRE | Пояснення |
|---------|-----|-------|-----------|
| Avg Latency | ~50ms | ~150ms | mTLS handshake дорогий |
| Connection Time | ~10ms | ~80ms | Mutual TLS validation |
| Bytes per req | 1.5 KB | 3 KB | Certificate exchange |

#### Метрики для збереження:
- [ ] Screenshot з різницею в connection time
- [ ] Export: http_req_connecting, http_req_tls_handshaking
- [ ] Зберегти JSON results

---

### Scenario 3: Keep-Alive Efficiency
**Час виконання:** 30 хв × 2 стенди = 60 хв

#### Особливість:
✅ **Keep-Alive увімкнено!** Мало VU, багато запитів на з'єднання

#### Команди:
```bash
# JWT
k6 run -e BASE_URL=$JWT_URL -e AUTH_TYPE=JWT -e JWT_TOKEN=$JWT_TOKEN \
  scenario3-keepalive-efficiency.js

sleep 1h

# SPIRE
k6 run -e BASE_URL=$SPIRE_URL -e AUTH_TYPE=SPIRE \
  scenario3-keepalive-efficiency.js
```

#### Що спостерігати в Grafana:
- [ ] **"Spring Security Latency"** - JWT має бути вищим
- [ ] У JWT постійна валідація токенів
- [ ] У SPIRE валідація була тільки при handshake
- [ ] CPU Usage - JWT споживає більше на валідацію

#### Очікувані результати:
| Метрика | JWT | SPIRE | Пояснення |
|---------|-----|-------|-----------|
| Security Overhead | 2-5ms | ~0ms | JWT перевіряється кожен раз |
| CPU (process) | 15% | 8% | Криптографія JWT |
| Total latency | ~70ms | ~50ms | Накладні витрати JWT |

#### Ключовий інсайт:
> **Для 10,000 запитів:**
> - JWT витрачає: 10,000 × 3ms = 30 секунд на валідацію
> - SPIRE витрачає: 10 × 80ms = 0.8 секунд (тільки handshake)
> - **Економія: ~96%** часу на автентифікацію!

#### Метрики для збереження:
- [ ] Screenshot "Spring Security Latency" панелі
- [ ] Export: spring_security_filterchains_seconds
- [ ] CSV з CPU usage
- [ ] JSON results

---

### Scenario 4: Network Payload Analysis
**Час виконання:** 15 хв × 2 стенди = 30 хв

#### Особливість:
📦 Мінімальний payload (порожній JSON), максимум overhead

#### Команди:
```bash
# JWT
k6 run -e BASE_URL=$JWT_URL -e AUTH_TYPE=JWT -e JWT_TOKEN=$JWT_TOKEN \
  scenario4-network-payload.js

sleep 30m

# SPIRE
k6 run -e BASE_URL=$SPIRE_URL -e AUTH_TYPE=SPIRE \
  scenario4-network-payload.js
```

#### Що спостерігати в Grafana:
- [ ] **"Bytes per Request"** панель
- [ ] Network Throughput (MB/s)
- [ ] JWT додає ~500-2000 байт на кожен запит
- [ ] SPIRE overhead = 0 байт

#### Розрахунки для роботи:
```
При 1,000,000 запитів/день:

JWT Token Size: 1500 bytes
JWT overhead: 1500 bytes × 1,000,000 = 1.5 GB/day
SPIRE overhead: 0 bytes × 1,000,000 = 0 GB/day

Вартість (AWS $0.09/GB):
- JWT: $0.135/day = $4.05/month
- SPIRE: $0/month
- Економія: $48.60/рік
```

#### Метрики для збереження:
- [ ] Screenshot "Bytes per Request" comparison
- [ ] Export: container_network_transmit_bytes_total
- [ ] Калькуляція вартості з JSON results
- [ ] Таблиця з розрахунками

---

### Scenario 5: Identity Issuance Stress
**Час виконання:** 15 хв (тільки SPIRE)

#### Особливість:
⚡ Тестує масштабованість SPIRE під час deploy

#### Підготовка:
```bash
# Переконайтеся що у вас є kubectl доступ
kubectl get pods -n default

# Знайдіть deployment name
kubectl get deployments
```

#### Команди:
```bash
# Запустіть тест
k6 run -e BASE_URL=$SPIRE_URL -e AUTH_TYPE=SPIRE \
  -e K8S_NAMESPACE=default \
  -e K8S_DEPLOYMENT=service-spire \
  scenario5-identity-stress.js &

# На 5й хвилині виконайте scale-up
sleep 5m
kubectl scale deployment/service-spire --replicas=5

# Спостерігайте за метриками ще 10 хвилин
```

#### Що спостерігати в Grafana:
- [ ] **"SVIDs Issued Rate"** - має сплеснути до ~5 ops/s
- [ ] **"Workload Attestation Latency"** - може зрости до 50-100ms
- [ ] "Application Ready Time" - як швидко нові pods стають ready
- [ ] "SPIRE Agent CPU" - навантаження на інфраструктуру

#### Таймінги для документування:
```
T+0:00 - Baseline (1 replica, 50 RPS)
T+5:00 - Scale-up initiated (kubectl scale)
T+5:15 - New pods starting (SVID issuance spike)
T+6:00 - All pods ready
T+15:00 - Test complete
```

#### Метрики для збереження:
- [ ] Screenshot SPIRE Internals panel під час spike
- [ ] Export: spire_server_rpc_svid_v1_svid_batch_new_x509svid
- [ ] Записати час від scale до ready state
- [ ] JSON results

---

## 📊 Після кожного тесту

### Immediate Actions:
1. [ ] Зберегти JSON результат k6
2. [ ] Screenshot Grafana dashboard
3. [ ] Експортувати CSV з ключових панелей
4. [ ] Записати несподівані спостереження

### Export з Grafana:
```bash
# Метод 1: UI
# Dashboard → Share → Export → Save to file

# Метод 2: API
curl -H "Authorization: Bearer $GRAFANA_API_KEY" \
  "http://localhost:3000/api/dashboards/uid/jwt-spire-by-instance" \
  | jq '.dashboard' > grafana-dashboard-export.json
```

### Export з Prometheus:
```bash
# Приклад для p99 latency
curl -G 'http://localhost:9090/api/v1/query_range' \
  --data-urlencode 'query=histogram_quantile(0.99, rate(http_server_requests_seconds_bucket[5m]))' \
  --data-urlencode 'start=2024-01-01T10:00:00Z' \
  --data-urlencode 'end=2024-01-01T10:20:00Z' \
  --data-urlencode 'step=15s' \
  > p99-latency-data.json
```

---

## 📝 Таблиця для заповнення результатів

| Scenario | Метрика | JWT | SPIRE | Δ (%) | Висновок |
|----------|---------|-----|-------|-------|----------|
| 1: Steady | p99 Latency (ms) | | | | |
| 1: Steady | Avg CPU (%) | | | | |
| 1: Steady | Memory (MB) | | | | |
| 2: Churn | Connection Time (ms) | | | | |
| 2: Churn | TLS Handshake (ms) | | | | |
| 3: KeepAlive | Security Overhead (ms) | | | | |
| 3: KeepAlive | Total CPU saved (%) | | | | |
| 4: Network | Bytes per Request | | | | |
| 4: Network | Cost per 1M req ($) | | | | |
| 5: Stress | SVID issue rate (ops/s) | N/A | | N/A | |
| 5: Stress | Time to ready (sec) | N/A | | N/A | |

---

## ⚠️ Troubleshooting

### k6 повертає помилки 5xx:
```bash
# Зменшіть RPS
k6 run --vus 20 ...  # замість 50

# Збільште timeout
k6 run --http-debug="full" ...
```

### Grafana не показує дані:
```bash
# Перевірте scrape
curl http://localhost:9090/api/v1/targets

# Перевірте метрики
curl http://localhost:8080/actuator/prometheus | grep http_server
```

### JWT токен expired:
```bash
# Отримайте новий
export JWT_TOKEN=$(curl ... | jq -r '.token')
```

---

## 🎓 Для магістерської роботи

### Обов'язкові deliverables:
1. [ ] Таблиця з усіма метриками (як вище)
2. [ ] 10-15 screenshots з Grafana
3. [ ] Графіки порівняння (використайте analyze_results.py)
4. [ ] Розділ "Методологія" з описом кожного сценарію
5. [ ] Розділ "Результати" з аналізом кожної метрики
6. [ ] Висновки про trade-offs JWT vs SPIRE

### Структура розділу "Експериментальне дослідження":
```
4. Експериментальне дослідження
   4.1 Методологія
       4.1.1 Опис тестових стендів
       4.1.2 Інструменти моніторингу
       4.1.3 Сценарії тестування
   4.2 Сценарій 1: Steady State Baseline
       4.2.1 Постановка експерименту
       4.2.2 Результати
       4.2.3 Аналіз
   4.3 Сценарій 2: Connection Churn
       ...
   4.7 Агрегований аналіз
   4.8 Висновки
```

---

## ✅ Фінальний чек-лист

- [ ] Всі 5 сценаріїв виконані для обох стендів
- [ ] Зібрані JSON результати k6
- [ ] Зібрані CSV з Prometheus
- [ ] Screenshots з Grafana збережені
- [ ] Таблиця результатів заповнена
- [ ] Графіки створені (analyze_results.py)
- [ ] Несподівані спостереження задокументовані
- [ ] Конфігурація стендів описана
- [ ] Trade-offs визначені
- [ ] Рекомендації сформульовані

**Успіхів! 🚀**
