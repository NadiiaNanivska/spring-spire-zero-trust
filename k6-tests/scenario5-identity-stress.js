// scenario5-identity-stress.js
// Сценарій 5: Горизонтальне масштабування під паралельним навантаженням
// Мета: Перевірити, як система (додатки + SPIRE) поводиться, коли кількість
//       реплік змінюється (scale-out / scale-in) одночасно з живим потоком
//       паралельних запитів. Масштабування виконується автоматично супровідним
//       скриптом run-scenario5-scaling.sh, синхронізованим за часом з фазами нижче.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const phaseLatency = {
  baseline: new Trend('latency_baseline'),
  ramp_up: new Trend('latency_ramp_up'),
  scaling: new Trend('latency_scaling'),
  post_scale: new Trend('latency_post_scale'),
  scale_down: new Trend('latency_scale_down'),
  cooldown: new Trend('latency_cooldown'),
};

// --- Часові межі фаз (секунди від старту тесту) ---------------------------
// Мають збігатися з таймінгом kubectl scale у run-scenario5-scaling.sh
const BASELINE_RPS = Number(__ENV.BASELINE_RPS || 30);
const TARGET_RPS = Number(__ENV.TARGET_RPS || 150);

const T_BASELINE_END = 120;   // кінець базового навантаження (2 хв)
const T_RAMP_END = 180;       // кінець розгону навантаження (1 хв) -> тут стартує scale-up
const T_SCALING_END = 300;    // кінець "перехідної" фази after scale-up (2 хв)
const T_POST_SCALE_END = 540; // кінець стабільної фази на повних репліках (4 хв) -> тут стартує scale-down
const T_SCALE_DOWN_END = 600; // кінець фази зниження навантаження/реплік (1 хв)
// далі до кінця тесту (660s = 11 хв) - cooldown

export const options = {
  scenarios: {
    parallel_load: {
      executor: 'ramping-arrival-rate',
      startRate: BASELINE_RPS,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 300,
      stages: [
        { duration: '2m', target: BASELINE_RPS },  // baseline: система в стані спокою
        { duration: '1m', target: TARGET_RPS },    // ramp_up: навантаження росте -> тригер для scale-up
        { duration: '2m', target: TARGET_RPS },    // scaling: нові поди піднімаються під навантаженням
        { duration: '4m', target: TARGET_RPS },    // post_scale: стабільна робота на повних репліках
        { duration: '1m', target: BASELINE_RPS },  // scale_down: навантаження падає -> тригер для scale-in
        { duration: '1m', target: BASELINE_RPS },  // cooldown: система після зменшення реплік
      ],
    },
  },
  thresholds: {
    'http_req_duration{phase:baseline}': ['p(95)<500'],
    'http_req_duration{phase:ramp_up}': ['p(95)<800'],
    'http_req_duration{phase:scaling}': ['p(95)<1500'],
    'http_req_duration{phase:post_scale}': ['p(95)<500'],
    'http_req_duration{phase:scale_down}': ['p(95)<1000'],
    errors: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const AUTH_TYPE = __ENV.AUTH_TYPE || 'SPIRE';
const NAMESPACE = __ENV.K8S_NAMESPACE || 'spire';
const DEPLOYMENT = __ENV.K8S_DEPLOYMENT || 'payments-service';
const MIN_REPLICAS = Number(__ENV.MIN_REPLICAS || 1);
const MAX_REPLICAS = Number(__ENV.MAX_REPLICAS || 15);

const formatLocalTime = (date) => {
  const pad = (n) => n.toString().padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
};

// Визначає поточну фазу тесту на основі часу від старту (testStartTime передається через VU-shared env)
function getPhase(elapsedSec) {
  if (elapsedSec < T_BASELINE_END) return 'baseline';
  if (elapsedSec < T_RAMP_END) return 'ramp_up';
  if (elapsedSec < T_SCALING_END) return 'scaling';
  if (elapsedSec < T_POST_SCALE_END) return 'post_scale';
  if (elapsedSec < T_SCALE_DOWN_END) return 'scale_down';
  return 'cooldown';
}

const testStartMs = Date.now();

export default function () {
  const url = `${BASE_URL}/api/orders/create`;

  const payload = JSON.stringify({
    itemId: 'test-item-123',
    quantity: 1,
  });

  const elapsedSec = (Date.now() - testStartMs) / 1000;
  const phase = getPhase(elapsedSec);

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: {
      scenario: 'horizontal_scaling',
      auth_type: AUTH_TYPE,
      phase,
    },
  };

  const res = http.post(url, payload, params);

  const ok = check(res, {
    'status is 200 or 201': (r) => r.status === 200 || r.status === 201,
    'response time acceptable': (r) => r.timings.duration < 1500,
  });

  if (!ok) errorRate.add(1);
  if (phaseLatency[phase]) phaseLatency[phase].add(res.timings.duration);

  sleep(0.02);
}

export function setup() {
  const startTime = new Date();
  const rampUpTime = new Date(startTime.getTime() + T_RAMP_END * 1000);
  const scaleDownTime = new Date(startTime.getTime() + T_POST_SCALE_END * 1000);

  console.log('🚀 Starting Horizontal Scaling Under Parallel Load Test');
  console.log(`   Auth Type: ${AUTH_TYPE}`);
  console.log(`   Target: ${BASE_URL}`);
  console.log(`   Deployment: ${DEPLOYMENT} (namespace: ${NAMESPACE})`);
  console.log(`   Replicas: ${MIN_REPLICAS} -> ${MAX_REPLICAS} -> ${MIN_REPLICAS}`);

  console.log('\n⏱️  ХРОНОЛОГІЯ ФАЗ (ЛОКАЛЬНИЙ ЧАС):');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.log(`   ▶ Початок             : ${formatLocalTime(startTime)}`);
  console.log(`   📈 Ramp-up завершено   : ${formatLocalTime(rampUpTime)} (тут стартує scale-up)`);
  console.log(`   📉 Post-scale завершено: ${formatLocalTime(scaleDownTime)} (тут стартує scale-down)`);
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.log('\n⚠️  Це навантаження розраховане на СИНХРОННИЙ запуск з');
  console.log('   run-scenario5-scaling.sh, який автоматично виконає');
  console.log(`   kubectl scale deployment/${DEPLOYMENT} -n ${NAMESPACE} --replicas=${MAX_REPLICAS}`);
  console.log('   та зворотне масштабування у потрібні моменти.\n');
}

export function handleSummary(data) {
  console.log('\n⚡ Scenario 5: Horizontal Scaling Under Load - Results');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.log(`Auth Type: ${AUTH_TYPE}`);
  console.log(`Total requests: ${data.metrics.http_reqs.values.count}`);
  console.log(`Error rate: ${(data.metrics.errors.values.rate * 100).toFixed(2)}%`);

  const phases = ['baseline', 'ramp_up', 'scaling', 'post_scale', 'scale_down', 'cooldown'];
  console.log('\n📊 Latency за фазами:');
  const phaseResults = {};
  for (const phase of phases) {
    const metricName = `latency_${phase}`;
    const m = data.metrics[metricName];
    if (m && m.values.count > 0) {
      phaseResults[phase] = {
        count: m.values.count,
        avg: m.values.avg,
        p95: m.values['p(95)'],
      };
      console.log(`  ${phase.padEnd(12)} n=${m.values.count.toString().padEnd(6)} avg=${m.values.avg.toFixed(2)}ms  p95=${m.values['p(95)'].toFixed(2)}ms`);
    }
  }

  console.log('\n📈 Що дивитись у Grafana:');
  console.log('  ✓ Кількість готових реплік (kube_deployment_status_replicas_ready), якщо доступно');
  console.log('  ✓ "SVIDs Issued Rate" - сплеск SVID issuance під час scale-up');
  console.log('  ✓ "Workload Attestation Latency" - латентність атестації нових подів');
  console.log('  ✓ Латентність HTTP під час фаз scaling / scale_down - деградація/відновлення');
  console.log('  ✓ Помилки/таймаути під час scale-down (in-flight запити на подах, що завершуються)');

  console.log('\n💡 Питання для аналізу:');
  console.log('  • Наскільки зростає латентність під час фази "scaling" порівняно з "baseline"?');
  console.log('  • Чи повертається латентність до норми у фазі "post_scale"?');
  console.log('  • Скільки запитів завершуються помилкою під час "scale_down" (graceful shutdown)?');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');

  return {
    [`results-scenario5-${AUTH_TYPE}.json`]: JSON.stringify({
      scenario: 'Horizontal Scaling Under Parallel Load',
      auth_type: AUTH_TYPE,
      total_requests: data.metrics.http_reqs.values.count,
      error_rate: data.metrics.errors.values.rate,
      duration_sec: data.state.testRunDurationMs / 1000,
      phases: phaseResults,
    }, null, 2),
  };
}

export function teardown(data) {
  console.log('\n✅ Test Complete!');
  console.log(`   Переконайтесь, що масштабування повернулось до ${MIN_REPLICAS} реплік:`);
  console.log(`   kubectl scale deployment/${DEPLOYMENT} -n ${NAMESPACE} --replicas=${MIN_REPLICAS}`);
}
