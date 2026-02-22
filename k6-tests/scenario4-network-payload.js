// scenario4-network-payload.js
// Сценарій 4: Мережевий податок (Network Payload Analysis)
// Мета: Порівняти накладні витрати трафіку

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const requestSize = new Trend('request_size_bytes');
const responseSize = new Trend('response_size_bytes');

export const options = {
  scenarios: {
    network_test: {
      executor: 'constant-arrival-rate',
      rate: 100,
      timeUnit: '1s',
      duration: '15m',
      preAllocatedVUs: 50,
      maxVUs: 100,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'],
    errors: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const AUTH_TYPE = __ENV.AUTH_TYPE || 'JWT';
const JWT_TOKEN = __ENV.JWT_TOKEN || '';

export default function () {
  // МІНІМАЛЬНИЙ payload - порожній JSON
  const payload = JSON.stringify({});
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      ...(AUTH_TYPE === 'JWT' && JWT_TOKEN && { 
        'Authorization': `Bearer ${JWT_TOKEN}` 
      }),
    },
    tags: { 
      scenario: 'network_payload',
      auth_type: AUTH_TYPE 
    },
  };

  // POST запит з мінімальним payload
  const res = http.post(`${BASE_URL}/api/v1/data`, payload, params);

  // Розрахунок розміру запиту
  let reqSize = payload.length;
  
  // Додаємо розмір заголовків
  Object.keys(params.headers).forEach(key => {
    reqSize += key.length + params.headers[key].length + 4; // +4 для ": " і "\r\n"
  });
  
  // Додаємо базові HTTP заголовки (~100 bytes)
  reqSize += 100;
  
  requestSize.add(reqSize);
  responseSize.add(res.body ? res.body.length : 0);

  check(res, {
    'status is 200 or 201': (r) => r.status === 200 || r.status === 201,
  }) || errorRate.add(1);

  sleep(0.01);
}

export function handleSummary(data) {
  const totalRequests = data.metrics.http_reqs.values.count;
  const avgReqSize = data.metrics.request_size_bytes.values.avg;
  const avgRespSize = data.metrics.response_size_bytes.values.avg;
  
  // Розрахунки
  const totalTrafficMB = ((avgReqSize + avgRespSize) * totalRequests) / (1024 * 1024);
  const hourlyTrafficGB = (totalTrafficMB / (data.state.testRunDurationMs / 1000 / 3600)) / 1024;
  
  // Оцінка розміру JWT токена (якщо є)
  let jwtOverheadBytes = 0;
  if (AUTH_TYPE === 'JWT' && JWT_TOKEN) {
    jwtOverheadBytes = JWT_TOKEN.length;
  }
  
  const jwtTaxPercentage = (jwtOverheadBytes / avgReqSize) * 100;
  
  console.log('\n📦 Scenario 4: Network Payload Analysis');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.log(`Auth Type: ${AUTH_TYPE}`);
  console.log(`Total Requests: ${totalRequests.toLocaleString()}`);
  console.log(`\nAverage Sizes:`);
  console.log(`  Request: ${avgReqSize.toFixed(0)} bytes`);
  console.log(`  Response: ${avgRespSize.toFixed(0)} bytes`);
  console.log(`  Total per request: ${(avgReqSize + avgRespSize).toFixed(0)} bytes`);
  
  if (AUTH_TYPE === 'JWT' && jwtOverheadBytes > 0) {
    console.log(`\n🎫 JWT Token Overhead:`);
    console.log(`  Token size: ${jwtOverheadBytes} bytes`);
    console.log(`  Percentage of request: ${jwtTaxPercentage.toFixed(1)}%`);
    console.log(`  Waste per 1M requests: ${(jwtOverheadBytes * 1000000 / 1024 / 1024).toFixed(2)} MB`);
  }
  
  console.log(`\n📊 Traffic Projection:`);
  console.log(`  Total traffic: ${totalTrafficMB.toFixed(2)} MB`);
  console.log(`  Projected hourly: ${hourlyTrafficGB.toFixed(3)} GB/h`);
  console.log(`  Projected daily: ${(hourlyTrafficGB * 24).toFixed(2)} GB/day`);
  console.log(`\n💰 Network Cost Estimation (at $0.09/GB):`);
  console.log(`  Hourly: $${(hourlyTrafficGB * 0.09).toFixed(4)}`);
  console.log(`  Daily: $${(hourlyTrafficGB * 24 * 0.09).toFixed(2)}`);
  console.log(`  Monthly: $${(hourlyTrafficGB * 24 * 30 * 0.09).toFixed(2)}`);
  
  console.log(`\n🎯 KEY INSIGHT:`);
  console.log(`  SPIRE: JWT headers відсутні (0 bytes overhead)`);
  console.log(`  JWT: ~${jwtOverheadBytes} bytes на кожен запит`);
  console.log(`  Saving with SPIRE: ${((jwtOverheadBytes * totalRequests) / 1024 / 1024).toFixed(2)} MB`);
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');

  return {
    [`results-scenario4-${AUTH_TYPE}.json`]: JSON.stringify({
      scenario: 'Network Payload Analysis',
      auth_type: AUTH_TYPE,
      total_requests: totalRequests,
      avg_request_size: avgReqSize,
      avg_response_size: avgRespSize,
      jwt_overhead_bytes: jwtOverheadBytes,
      jwt_tax_percentage: jwtTaxPercentage,
      total_traffic_mb: totalTrafficMB,
      hourly_traffic_gb: hourlyTrafficGB,
      daily_traffic_gb: hourlyTrafficGB * 24,
      monthly_cost_usd: hourlyTrafficGB * 24 * 30 * 0.09,
    }, null, 2),
  };
}
