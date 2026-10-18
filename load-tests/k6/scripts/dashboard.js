// dashboard.js — 50 concurrent users hitting GET /dashboard/:year/:month
//
// NOTE: The auth endpoint has a rate limit of 5 req/min per IP.
// This script registers 5 users (one batch) and distributes tokens round-robin
// across all 50 VUs. To test with more unique users, increase the rate limit
// in cmd/api/main.go or pass pre-created tokens via BASE_URL + TOKEN_* env vars.
//
// Run:
//   k6 run load-tests/k6/scripts/dashboard.js --out json=load-tests/k6/reports/dashboard.json
//
// Or via Makefile:
//   make load-test-dashboard

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
    console.error(`Registration failed: ${res.status} ${res.body}`);
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

// setup() registers up to 5 users (rate-limit budget) and shares them across VUs.
export function setup() {
  const tokens = [];
  for (let i = 0; i < 5; i++) {
    const tok = registerOne();
    if (tok) tokens.push(tok);
    sleep(0.2);
  }
  if (tokens.length === 0) {
    throw new Error('Could not register any test users — is the server running?');
  }
  console.log(`Registered ${tokens.length} users; sharing tokens across 50 VUs.`);
  return { tokens };
}

export const options = {
  vus: 50,
  duration: '30s',
  thresholds: {
    http_req_duration: ['p(95)<500'],
    // tag only 5xx as failures — 429 on auth is expected if re-running quickly
    'http_req_failed{scenario:default}': ['rate==0'],
  },
};

export default function (data) {
  const vu = __VU - 1;
  const token = data.tokens[vu % data.tokens.length];

  const res = http.get(
    `${BASE_URL}/v1/dashboard/2026/10`,
    authHeaders(token),
  );

  check(res, {
    'dashboard 200': (r) => r.status === 200,
    'has income': (r) => r.json('income') !== undefined,
    'has expenses': (r) => r.json('expenses') !== undefined,
  });

  sleep(0.1);
}
