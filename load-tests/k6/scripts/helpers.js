// helpers.js — shared utilities for Lastro load tests

import http from 'k6/http';
import { check } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

/**
 * Register a new user and return { token, userId }.
 * Uses uuidv4 to guarantee globally unique emails.
 */
export function registerAndLogin() {
  const email = `lt_${uuidv4()}@test.com`;
  const password = 'Senha123!';

  const reg = http.post(
    `${BASE_URL}/v1/auth/register`,
    JSON.stringify({ name: 'Load Test', email, password }),
    { headers: { 'Content-Type': 'application/json' } },
  );

  check(reg, { 'register 201': (r) => r.status === 201 });

  const body = reg.json();
  return { token: body.access_token, userId: body.user?.id, email, password };
}

export function authHeaders(token) {
  return {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  };
}

export { BASE_URL };
