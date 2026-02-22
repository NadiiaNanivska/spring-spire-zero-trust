// scenario2-connection-churn.js
// Сценарій 2: Ціна нового з'єднання (Connection Churn)
// Мета: Показати слабку сторону mTLS — важкий Handshake

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const handshakeTime = new Trend('tls_handshake_time');

export const options = {
  scenarios: {
    connection_churn: {
      executor: 'constant-arrival-rate',
      rate: 50,               // 50 RPS (менше ніж steady state)
      timeUnit: '1s',
      duration: '10m',
      preAllocatedVUs: 30,
      maxVUs: 60,
    },
  },
  // КРИТИЧНО: Вимикаємо Keep-Alive!
  noConnectionReuse: true,
  insecureSkipTLSVerify: false, // Перевіряємо TLS сертифікати
  thresholds: {
    http_req_duration: ['p(95)<1000', 'p(99)<2000'],
    http_req_connecting: ['p(95)<500'], // Час встановлення з'єднання
    errors: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const AUTH_TYPE = __ENV.AUTH_TYPE || 'JWT';
const JWT_TOKEN = __ENV.JWT_TOKEN || '';

export default function () {
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Connection': 'close', // ЯВНО закриваємо з'єднання
      ...(AUTH_TYPE === 'JWT' && JWT_TOKEN && { 
        'Authorization': `Bearer ${JWT_TOKEN}` 
      }),
    },
    tags: { 
      scenario: 'connection_churn',
      auth_type: AUTH_TYPE 
    },
  };

  const res = http.get(`${BASE_URL}/api/v1/data`, params);

  // Записуємо час TLS handshake
  if (res.timings.tls_handshaking) {
    handshakeTime.add(res.timings.tls_handshaking);
  }

  check(res, {
    'status is 200': (r) => r.status === 200,
    'TLS handshake completed': (r) => r.timings.tls_handshaking !== undefined,
    'connection established': (r) => r.timings.connecting > 0,
  }) || errorRate.add(1);

  sleep(0.02); // Невеликий sleep між запитами
}

export function handleSummary(data) {
  const summary = {
    scenario: 'Connection Churn',
    auth_type: AUTH_TYPE,
    duration_sec: data.state.testRunDurationMs / 1000,
    total_requests: data.metrics.http_reqs.values.count,
    rps: data.metrics.http_reqs.values.rate,
    latency: {
      avg: data.metrics.http_req_duration.values.avg,
      p50: data.metrics.http_req_duration.values['p(50)'],
      p95: data.metrics.http_req_duration.values['p(95)'],
      p99: data.metrics.http_req_duration.values['p(99)'],
    },
    connection_time: {
      avg: data.metrics.http_req_connecting?.values.avg || 0,
      p95: data.metrics.http_req_connecting?.values['p(95)'] || 0,
    },
    tls_handshake: {
      avg: data.metrics.tls_handshake_time?.values.avg || 0,
      p95: data.metrics.tls_handshake_time?.values['p(95)'] || 0,
    },
    error_rate: data.metrics.errors.values.rate,
  };

  console.log('\n🔥 Scenario 2: Connection Churn Results');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.log(`Auth Type: ${AUTH_TYPE}`);
  console.log(`Total Requests: ${summary.total_requests}`);
  console.log(`\nLatency:`);
  console.log(`  avg: ${summary.latency.avg.toFixed(2)}ms`);
  console.log(`  p95: ${summary.latency.p95.toFixed(2)}ms`);
  console.log(`  p99: ${summary.latency.p99.toFixed(2)}ms`);
  console.log(`\nConnection Time:`);
  console.log(`  avg: ${summary.connection_time.avg.toFixed(2)}ms`);
  console.log(`  p95: ${summary.connection_time.p95.toFixed(2)}ms`);
  console.log(`\nTLS Handshake:`);
  console.log(`  avg: ${summary.tls_handshake.avg.toFixed(2)}ms`);
  console.log(`  p95: ${summary.tls_handshake.p95.toFixed(2)}ms`);
  console.log(`\n⚠️  КЛЮЧОВИЙ ІНСАЙТ:`);
  console.log(`   SPIRE має показати ВИЩИЙ connection time через mTLS handshake`);
  console.log(`   JWT має показати НИЖЧИЙ connection time (звичайний TLS)`);
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');

  return {
    [`results-scenario2-${AUTH_TYPE}.json`]: JSON.stringify(summary, null, 2),
  };
}
