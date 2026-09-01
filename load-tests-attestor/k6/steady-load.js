import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

const rate = Number(__ENV.RATE || 100);
const duration = __ENV.DURATION || '10m';
const baseUrl = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  scenarios: {
    steady: {
      executor: 'constant-arrival-rate',
      rate,
      timeUnit: '1s',
      duration,
      preAllocatedVUs: Math.min(rate, 50),
      maxVUs: Math.max(rate * 2, 100),
    },
  },
  thresholds: {
    errors: ['rate<0.05'],
  },
};

export default function () {
  const payload = JSON.stringify({ itemId: 'attestor-load-test', quantity: 1 });
  const res = http.post(`${baseUrl}/api/orders/create`, payload, {
    headers: { 'Content-Type': 'application/json' },
    tags: { scenario: 'steady_load' },
  });
  const ok = check(res, {
    'status 200 or 201': (r) => r.status === 200 || r.status === 201,
  });
  if (!ok) errorRate.add(1);
  sleep(0.01);
}

export function handleSummary(data) {
  // Emit the summary on stdout between markers so it can be recovered from
  // pod logs when k6 runs in-cluster (the k6 image has no tar for kubectl cp).
  const oneLine = JSON.stringify(data);
  return {
    stdout: `\n__K6_SUMMARY_BEGIN__${oneLine}__K6_SUMMARY_END__\n`,
    'results-steady-load.json': JSON.stringify(data, null, 2),
  };
}
