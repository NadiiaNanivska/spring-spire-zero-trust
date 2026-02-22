// scenario3-keepalive-efficiency.js
// Сценарій 3: Ефективність у довгих сесіях (Keep-Alive Efficiency)
// Мета: Показати сильну сторону SPIRE — відсутність повторної валідації

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter } from 'k6/metrics';

const errorRate = new Rate('errors');
const requestsPerConnection = new Counter('requests_per_connection');

export const options = {
  scenarios: {
    keepalive_test: {
      executor: 'per-vu-iterations',
      vus: 10,                // Тільки 10 VU (постійні з'єднання)
      iterations: 1000,       // Кожен VU виконує 1000 запитів
      maxDuration: '30m',
    },
  },
  // КРИТИЧНО: Увімкнути Keep-Alive!
  noConnectionReuse: false,
  // HTTP/1.1 з Keep-Alive
  batch: 1,
  thresholds: {
    http_req_duration: ['p(95)<200', 'p(99)<300'],
    errors: ['rate<0.001'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const AUTH_TYPE = __ENV.AUTH_TYPE || 'JWT';
const JWT_TOKEN = __ENV.JWT_TOKEN || '';

export default function () {
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Connection': 'keep-alive', // ЯВНО вказуємо Keep-Alive
      ...(AUTH_TYPE === 'JWT' && JWT_TOKEN && { 
        'Authorization': `Bearer ${JWT_TOKEN}` 
      }),
    },
    tags: { 
      scenario: 'keepalive_efficiency',
      auth_type: AUTH_TYPE,
      vu: __VU,
    },
  };

  const res = http.get(`${BASE_URL}/api/v1/data`, params);

  requestsPerConnection.add(1);

  check(res, {
    'status is 200': (r) => r.status === 200,
    'connection reused': (r) => r.timings.connecting === 0, // 0 = reused
    'low latency': (r) => r.timings.duration < 150,
  }) || errorRate.add(1);

  // Дуже короткий sleep - імітація інтенсивного трафіку
  sleep(0.001);
}

export function handleSummary(data) {
  const totalRequests = data.metrics.http_reqs.values.count;
  const avgLatency = data.metrics.http_req_duration.values.avg;
  const connectingTime = data.metrics.http_req_connecting?.values.avg || 0;
  
  // Оцінка переваги SPIRE
  const theoreticalJwtOverhead = totalRequests * 2; // ~2ms на JWT валідацію
  const actualOverhead = avgLatency * totalRequests;
  
  console.log('\n✨ Scenario 3: Keep-Alive Efficiency Results');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.log(`Auth Type: ${AUTH_TYPE}`);
  console.log(`Total Requests: ${totalRequests}`);
  console.log(`VUs: ${data.options.scenarios.keepalive_test.vus}`);
  console.log(`Requests per VU: ${totalRequests / data.options.scenarios.keepalive_test.vus}`);
  console.log(`\nLatency:`);
  console.log(`  avg: ${avgLatency.toFixed(2)}ms`);
  console.log(`  p50: ${data.metrics.http_req_duration.values['p(50)'].toFixed(2)}ms`);
  console.log(`  p95: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms`);
  console.log(`  p99: ${data.metrics.http_req_duration.values['p(99)'].toFixed(2)}ms`);
  console.log(`\nConnection Efficiency:`);
  console.log(`  avg connecting time: ${connectingTime.toFixed(2)}ms`);
  console.log(`  (близько до 0 = з'єднання перевикористовуються)`);
  console.log(`\n📊 ОЧІКУВАННЯ:`);
  console.log(`  JWT: Стабільний Security Overhead ~1-5ms на кожен запит`);
  console.log(`  SPIRE: Security Overhead ~0ms (валідація при handshake)`);
  console.log(`\n💡 Для ${totalRequests.toLocaleString()} запитів:`);
  console.log(`  JWT теоретична втрата: ~${theoreticalJwtOverhead.toFixed(0)}ms`);
  console.log(`  SPIRE теоретична перевага: ~${(theoreticalJwtOverhead / 1000).toFixed(1)}s`);
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');

  return {
    [`results-scenario3-${AUTH_TYPE}.json`]: JSON.stringify({
      scenario: 'Keep-Alive Efficiency',
      auth_type: AUTH_TYPE,
      total_requests: totalRequests,
      vus: data.options.scenarios.keepalive_test.vus,
      latency: {
        avg: avgLatency,
        p50: data.metrics.http_req_duration.values['p(50)'],
        p95: data.metrics.http_req_duration.values['p(95)'],
        p99: data.metrics.http_req_duration.values['p(99)'],
      },
      connection_time_avg: connectingTime,
      error_rate: data.metrics.errors.values.rate,
    }, null, 2),
  };
}
