// idempotency.js — 10 threads sending the same client_id simultaneously.
//
// Goal: verify that only ONE transaction is persisted regardless of concurrent
// duplicate submissions (enforced by the UNIQUE constraint on client_id).
//
// Run:
//   k6 run load-tests/k6/scripts/idempotency.js --out json=load-tests/k6/reports/idempotency.json
//
// Or via Makefile:
//   make load-test-idempotency
//
// After the run, verify in the database:
//   SELECT COUNT(*) FROM transactions
//   WHERE client_id = '<SHARED_CLIENT_ID>' AND deleted_at IS NULL;
//   -- must be exactly 1

import http from 'k6/http';
import { check } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

// Treat 4xx as "expected" so http_req_failed only counts 5xx errors.
http.setResponseCallback(http.expectedStatuses({ min: 200, max: 499 }));

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const SHARED_CLIENT_ID =
  __ENV.SHARED_CLIENT_ID || 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee';

function authHeaders(token) {
  return {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  };
}

export function setup() {
  const email = `lt_idm_${uuidv4()}@test.com`;
  const res = http.post(
    `${BASE_URL}/v1/auth/register`,
    JSON.stringify({ name: 'Load Test', email, password: 'Senha123!' }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  if (res.status !== 201) {
    throw new Error(`Could not register user: ${res.status} ${res.body}`);
  }
  const token = res.json().access_token;
  console.log(`Registered user; shared client_id: ${SHARED_CLIENT_ID}`);
  return { token };
}

export const options = {
  // All 10 VUs start at the same time; each sends exactly one request.
  vus: 10,
  iterations: 10,
  thresholds: {
    http_req_duration: ['p(95)<500'],
    // Only 409 Conflict or 201 Created acceptable — never 5xx.
    'http_req_failed{scenario:default}': ['rate==0'],
  },
};

export default function (data) {
  const res = http.post(
    `${BASE_URL}/v1/transactions`,
    JSON.stringify({
      client_id: SHARED_CLIENT_ID,
      amount: 10,
      direction: 'out',
      date: '2026-10-01',
    }),
    authHeaders(data.token),
  );

  check(res, {
    'accepted or duplicate': (r) => r.status === 201 || r.status === 409,
    'no 5xx': (r) => r.status < 500,
  });
}

export function handleSummary(summary) {
  // Print DB verification query to stdout; return default summary to stderr.
  console.log(`\nIdempotency check — client_id used: ${SHARED_CLIENT_ID}`);
  console.log(
    `Verify: SELECT COUNT(*) FROM transactions WHERE client_id = '${SHARED_CLIENT_ID}' AND deleted_at IS NULL;`,
  );
  console.log('Expected result: exactly 1 row.\n');
  // Return the default k6 summary to stdout as well.
  return { stdout: '\n' };
}
