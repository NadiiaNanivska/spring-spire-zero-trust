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
      duration: '15m',
      preAllocatedVUs: 30,
      maxVUs: 60,
      tags: { scenario_type: 'baseline' },
    },
    // Сплеск під час scale-up (запускається на 5й хвилині)
    scale_spike: {
      executor: 'ramping-arrival-rate',
      startRate: 0,
      timeUnit: '1s',
      preAllocatedVUs: 10,
      maxVUs: 30,
      stages: [
        { duration: '5m', target: 0 },    // Чекаємо 5 хв
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
const NAMESPACE = __ENV.K8S_NAMESPACE || 'default';
const DEPLOYMENT = __ENV.K8S_DEPLOYMENT || 'service-spire';

export default function () {
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

  const res = http.get(`${BASE_URL}/api/v1/health`, params);

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time acceptable': (r) => r.timings.duration < 1000,
  }) || errorRate.add(1);

  sleep(0.02);
}

// Setup function - запускається перед тестом
export function setup() {
  console.log('🚀 Starting Identity Issuance Stress Test');
  console.log(`   Auth Type: ${AUTH_TYPE}`);
  console.log(`   Target: ${BASE_URL}`);
  console.log('');
  console.log('📝 Manual Steps Required:');
  console.log('   1. Monitor Grafana Dashboard');
  console.log('   2. At 5:00 mark, run: kubectl scale deployment/' + DEPLOYMENT + ' --replicas=5');
  console.log('   3. Observe SPIRE metrics:');
  console.log('      - SVIDs Issued Rate spike');
  console.log('      - Workload Attestation Latency');
  console.log('      - Application Ready Time');
  console.log('');
}

export function handleSummary(data) {
  console.log('\n⚡ Scenario 5: Identity Issuance Stress Results');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.log(`Auth Type: ${AUTH_TYPE}`);
  
  // Базове навантаження
  const baselineMetrics = Object.keys(data.metrics)
    .filter(k => k.includes('scenario_type:baseline'))
    .reduce((acc, k) => {
      acc[k] = data.metrics[k];
      return acc;
    }, {});
  
  // Spike metrics
  const spikeMetrics = Object.keys(data.metrics)
    .filter(k => k.includes('scenario_type:spike'))
    .reduce((acc, k) => {
      acc[k] = data.metrics[k];
      return acc;
    }, {});
  
  console.log('\n📊 Baseline Load (first 5 min):');
  console.log(`  Total requests: ${data.metrics.http_reqs.values.count}`);
  console.log(`  avg latency: ${data.metrics.http_req_duration.values.avg.toFixed(2)}ms`);
  console.log(`  p95 latency: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms`);
  
  console.log('\n🔥 During Scale-Up Spike:');
  console.log(`  Peak RPS: 100`);
  console.log(`  Expected behavior:`);
  console.log(`    - SPIRE: Temporary spike in attestation latency`);
  console.log(`    - SPIRE: SVIDs issued rate peaks`);
  console.log(`    - Application ready time increases`);
  
  console.log('\n📈 What to Check in Grafana:');
  console.log(`  ✓ "SVIDs Issued Rate" panel - spike at scale event`);
  console.log(`  ✓ "Workload Attestation Latency" - temporary increase`);
  console.log(`  ✓ "Application Ready Time" - deployment speed`);
  console.log(`  ✓ "SPIRE Agent CPU" - infrastructure overhead`);
  
  console.log('\n💡 Analysis Points:');
  console.log(`  • How long did it take for new pods to become ready?`);
  console.log(`  • Did SPIRE Server become a bottleneck?`);
  console.log(`  • Did existing pods experience latency degradation?`);
  console.log(`  • What's the SVID issuance capacity of your SPIRE setup?`);
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');

  return {
    [`results-scenario5-${AUTH_TYPE}.json`]: JSON.stringify({
      scenario: 'Identity Issuance Stress',
      auth_type: AUTH_TYPE,
      total_requests: data.metrics.http_reqs.values.count,
      avg_latency: data.metrics.http_req_duration.values.avg,
      p95_latency: data.metrics.http_req_duration.values['p(95)'],
      p99_latency: data.metrics.http_req_duration.values['p(99)'],
      error_rate: data.metrics.errors.values.rate,
      duration_sec: data.state.testRunDurationMs / 1000,
    }, null, 2),
  };
}

export function teardown(data) {
  console.log('\n✅ Test Complete!');
  console.log('   Remember to scale down: kubectl scale deployment/' + DEPLOYMENT + ' --replicas=1');
}
