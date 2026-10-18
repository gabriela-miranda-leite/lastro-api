// transactions.js — 20 concurrent users creating transactions (POST /transactions)
//
// Run:
//   k6 run load-tests/k6/scripts/transactions.js --out json=load-tests/k6/reports/transactions.json
//
// Or via Makefile:
//   make load-test-transactions

import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

function registerOne() {
  const email = `lt_${uuidv4()}@test.com`;
  const res = http.post(
    `${BASE_URL}/v1/auth/register`,
    JSON.stringify({ name: 'Load Test', email, password: 'Senha123!' }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  if (res.status !== 201) {
    console.error(`Registration failed: ${res.status}`);
    return null;
  }
  return res.json().access_token;
}

function authHeaders(token) {
  return {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  };
}

export function setup() {
  const tokens = [];
  for (let i = 0; i < 5; i++) {
    const tok = registerOne();
    if (tok) tokens.push(tok);
    sleep(0.2);
  }
  if (tokens.length === 0) {
    throw new Error('Could not register any test users.');
  }
  console.log(`Registered ${tokens.length} users; sharing across 20 VUs.`);
  return { tokens };
}

export const options = {
  vus: 20,
  duration: '30s',
  thresholds: {
    http_req_duration: ['p(95)<500'],
    'http_req_failed{scenario:default}': ['rate==0'],
  },
};

export default function (data) {
  const vu = __VU - 1;
  const token = data.tokens[vu % data.tokens.length];

  const res = http.post(
    `${BASE_URL}/v1/transactions`,
    JSON.stringify({
      client_id: uuidv4(),
      amount: 99.9,
      direction: 'out',
      date: '2026-10-01',
    }),
    authHeaders(token),
  );

  check(res, {
    'transaction 201': (r) => r.status === 201,
    'has id': (r) => r.json('id') !== undefined,
  });

  sleep(0.05);
}
