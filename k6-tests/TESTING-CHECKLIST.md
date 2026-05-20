# 📋 JWT vs SPIRE Testing Checklist
## Проведення експериментів

---

## 🎯 Загальна мета
Порівняти накладні витрати JWT та SPIRE для service-to-service автентифікації через 5 сценаріїв тестування.

---

## 📅 Розклад тестування (5 днів)

### День 1: Підготовка
- [x] Встановити k6: `brew install k6` або https://k6.io/docs/get-started/installation/
- [x] Перевірити доступність обох стендів
- [x] Налаштувати Grafana dashboards
- [x] Отримати JWT токен (якщо потрібно)
- [x] Зробити baseline screenshots (idle state)

### День 2: Scenario 1
- [x] Запустити Scenario 1 для JWT (15 хв)
- [x] Експортувати метрики з Grafana
- [x] Пауза 30 хв
- [x] Запустити Scenario 1 для SPIRE (15 хв)
- [x] Експортувати метрики

### День 3: Scenario 2
- [x] Запустити Scenario 2 для JWT (15 хв)
- [x] Експортувати метрики з Grafana
- [x] Пауза 30 хв
- [x] Запустити Scenario 2 для SPIRE (15 хв)
- [x] Експортувати метрики

### День 4: Scenario 3
- [x] Запустити Scenario 3 для JWT
- [x] Експортувати метрики з Grafana
- [x] Пауза 30 хв
- [x] Запустити Scenario 3 для SPIRE
- [x] Експортувати метрики

### День 5: Scenario 4
- [x] Запустити Scenario 1 для SPIRE з ротацією X.509 кожні 2,5хв (10 хв)
- [x] Експортувати метрики

### День 6: Scenario 5 + повтори
- [x] Запустити Scenario 5 (з kubectl scale)