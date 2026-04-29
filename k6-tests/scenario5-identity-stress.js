// scenario5-identity-stress.js
// Сценарій 5: Стрес-тест ідентичності (Identity Issuance Stress)
// Мета: Перевірити, чи не стає SPIRE вузьким місцем при масштабуванні

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';
import exec from 'k6/execution';

const errorRate = new Rate('errors');

export const options = {
  scenarios: {
    // Основне навантаження
    baseline_load: {
      executor: 'constant-arrival-rate',
      rate: 50,
      timeUnit: '1s',
      duration: '7m',
      preAllocatedVUs: 30,
      maxVUs: 60,
      tags: { scenario_type: 'baseline' },
    },
    // Сплеск під час scale-up (запускається на 3й хвилині)
    scale_spike: {
      executor: 'ramping-arrival-rate',
      startRate: 0,
      timeUnit: '1s',
      preAllocatedVUs: 10,
      maxVUs: 30,
      stages: [
        { duration: '3m', target: 0 },    // Чекаємо 3 хв
        { duration: '30s', target: 100 }, // Різкий сплеск
        { duration: '2m', target: 100 },  // Підтримуємо
        { duration: '1m', target: 0 },    // Зменшуємо
      ],
      tags: { scenario_type: 'spike' },
    },
  },
  thresholds: {
    'http_req_duration{scenario_type:baseline}': ['p(95)<500'],
    'http_req_duration{scenario_type:spike}': ['p(95)<1000'],
    errors: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const AUTH_TYPE = __ENV.AUTH_TYPE || 'SPIRE';
const NAMESPACE = __ENV.K8S_NAMESPACE || 'spire';
const DEPLOYMENT = __ENV.K8S_DEPLOYMENT || 'payments-service';

// Функція для форматування локального часу
const formatLocalTime = (date) => {
  const pad = (n) => n.toString().padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
};

export default function () {
  const url = `${BASE_URL}/api/orders/create`;

  const payload = JSON.stringify({
                                   itemId: "test-item-123",
                                   quantity: 1
                                 });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: {
      scenario: 'identity_stress',
      auth_type: AUTH_TYPE,
      stage: exec.scenario.name,
    },
  };

  const res = http.post(url, payload, params);

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time acceptable': (r) => r.timings.duration < 1000,
  }) || errorRate.add(1);

  sleep(0.02);
}

// Setup function - запускається перед тестом
export function setup() {
  const startTime = new Date();
  const spikeTime = new Date(startTime.getTime() + 3 * 60000); // + 3 хвилини

  console.log('🚀 Starting Identity Issuance Stress Test');
  console.log(`   Auth Type: ${AUTH_TYPE}`);
  console.log(`   Target: ${BASE_URL}`);

  console.log('\n⏱️  ЧАСОВІ МІТКИ (ЛОКАЛЬНИЙ ЧАС):');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.log(`   ▶ Початок тесту   : ${formatLocalTime(startTime)}`);
  console.log(`   ⚡ Старт сплеску   : ${formatLocalTime(spikeTime)} (Орієнтир для масштабування)`);
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');

  // Витягуємо тільки години, хвилини та секунди для підказки
  const spikeTimeOnly = formatLocalTime(spikeTime).substring(11, 19);

  console.log('\n📝 Manual Steps Required:');
  console.log('   1. Monitor Grafana Dashboard');
  console.log(`   2. Рівно о ${spikeTimeOnly} виконайте в сусідньому терміналі:`);
  console.log(`      kubectl scale deployment/${DEPLOYMENT} -n ${NAMESPACE} --replicas=15`);
  console.log('   3. Observe SPIRE SVIDs Issued Rate & CPU');
  console.log('');
}

export function handleSummary(data) {
  const endTime = new Date();
  const startTime = new Date(endTime.getTime() - data.state.testRunDurationMs);
  const spikeTime = new Date(startTime.getTime() + 3 * 60000);

  console.log('\n⚡ Scenario 5: Identity Issuance Stress Results');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.log(`Auth Type: ${AUTH_TYPE}`);

  console.log('\n⏱️  ХРОНОЛОГІЯ ТЕСТУ (ЛОКАЛЬНИЙ ЧАС):');
  console.log(`   ▶ Початок : ${formatLocalTime(startTime)}`);
  console.log(`   ⚡ Сплеск  : ${formatLocalTime(spikeTime)}`);
  console.log(`   ⏹ Кінець  : ${formatLocalTime(endTime)}`);
  console.log(`   ⏱ Тривалість: ${(data.state.testRunDurationMs / 1000 / 60).toFixed(2)} хв`);
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');

  console.log('\n📊 Baseline Load (first 3 min):');
  console.log(`  Total requests: ${data.metrics.http_reqs.values.count}`);
  console.log(`  avg latency: ${data.metrics.http_req_duration.values.avg.toFixed(2)}ms`);

  if (data.metrics.http_req_duration.values['p(95)']) {
    console.log(`  p95 latency: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms`);
  }

  console.log('\n🔥 During Scale-Up Spike:');
  console.log(`  Peak RPS: 100`);
  console.log(`  Expected behavior:`);
  console.log(`    - SPIRE: Temporary spike in attestation latency`);
  console.log(`    - SPIRE: SVIDs issued rate peaks`);
  console.log(`    - Application ready time increases`);

  console.log('\n📈 What to Check in Grafana:');
  console.log(`  ✓ "SVIDs Issued Rate" panel - spike at scale event`);
  console.log(`  ✓ "Workload Attestation Latency" - temporary increase`);
  console.log(`  ✓ "SPIRE Agent/Server CPU" - infrastructure overhead`);

  console.log('\n💡 Analysis Points:');
  console.log(`  • How long did it take for new pods to become ready?`);
  console.log(`  • Did existing pods experience latency degradation?`);
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');

  return {
    [`results-scenario5-${AUTH_TYPE}.json`]: JSON.stringify({
                                                              scenario: 'Identity Issuance Stress',
                                                              auth_type: AUTH_TYPE,
                                                              total_requests: data.metrics.http_reqs.values.count,
                                                              avg_latency: data.metrics.http_req_duration.values.avg,
                                                              error_rate: data.metrics.errors.values.rate,
                                                              duration_sec: data.state.testRunDurationMs / 1000,
                                                            }, null, 2),
  };
}

export function teardown(data) {
  console.log('\n✅ Test Complete!');
  console.log(`   Remember to scale down: kubectl scale deployment/${DEPLOYMENT} -n ${NAMESPACE} --replicas=1`);
}