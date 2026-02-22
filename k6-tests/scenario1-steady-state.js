import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const customLatency = new Trend('custom_latency');

export const options = {
  scenarios: {
    steady_state: {
      executor: 'constant-arrival-rate',
      rate: 100,              // 100 RPS
      timeUnit: '1s',
      duration: '10m',        // Тривалість тесту
      preAllocatedVUs: 50,
      maxVUs: 100,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    errors: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const AUTH_TYPE = __ENV.AUTH_TYPE || 'JWT';
const JWT_TOKEN = __ENV.JWT_TOKEN || '';

export default function () {
  const url = `${BASE_URL}/api/orders/create`;

  // Тіло запиту (Payload)
  const payload = JSON.stringify({
    itemId: "test-item-123",
    quantity: 1
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      ...(AUTH_TYPE === 'JWT' && JWT_TOKEN && {
        'Authorization': `Bearer ${JWT_TOKEN}`
      }),
    },
    tags: {
      scenario: 'steady_state',
      auth_type: AUTH_TYPE
    },
  };

  const startTime = Date.now();

  // Використовуємо http.post замість http.get
  const res = http.post(url, payload, params);

  const duration = Date.now() - startTime;
  customLatency.add(duration);

  check(res, {
    'status is 200 or 201': (r) => r.status === 200 || r.status === 201,
    'latency < 200ms': (r) => r.timings.duration < 200,
  }) || errorRate.add(1);

  sleep(0.01);
}

export function handleSummary(data) {
  return {
    [`results-scenario1-${AUTH_TYPE}.json`]: JSON.stringify(data, null, 2),
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
  };
}

function textSummary(data, options) {
  const { indent = '', enableColors = false } = options || {};
  return `
${indent}📊 Scenario 1: Steady State Baseline (POST Create Order) - ${AUTH_TYPE}
${indent}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
${indent}Duration: ${data.state.testRunDurationMs / 1000}s
${indent}Requests: ${data.metrics.http_reqs.values.count}
${indent}RPS: ${data.metrics.http_reqs.values.rate.toFixed(2)}
${indent}
${indent}Latency:
${indent}  avg: ${data.metrics.http_req_duration.values.avg.toFixed(2)}ms
${indent}  p95: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms
${indent}  p99: ${data.metrics.http_req_duration.values['p(99)'].toFixed(2)}ms
${indent}
${indent}Error Rate: ${(data.metrics.errors.values.rate * 100).toFixed(2)}%
${indent}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  `;
}