// k6 run k6/mcp-load-test.js        (needs: make up && make run)
// Simulates bursty agent traffic with a real OAuth 2.1 token: initialize -> tools/call in a loop.
import http from 'k6/http';
import { check, sleep } from 'k6';

const MCP = __ENV.MCP_URL || 'http://localhost:8080/mcp';
const TOKEN_URL = __ENV.TOKEN_URL || 'http://localhost:8180/realms/orders/protocol/openid-connect/token';
const CLIENT_ID = __ENV.CLIENT_ID || 'orders-readonly';
const CLIENT_SECRET = __ENV.CLIENT_SECRET || 'orders-readonly-secret'; // demo realm secret

export const options = {
  scenarios: {
    agents: {
      executor: 'ramping-arrival-rate',
      startRate: 10,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 200,
      stages: [
        { duration: '30s', target: 50 },
        { duration: '1m', target: 200 }, // burst: agents chaining calls
        { duration: '30s', target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    'http_req_duration{tool:findOrder}': ['p(95)<150'],
    'http_req_duration{tool:listRecentOrders}': ['p(95)<200'],
  },
};

function headers(token, session) {
  const h = {
    'Content-Type': 'application/json',
    Accept: 'application/json, text/event-stream',
    Authorization: `Bearer ${token}`,
  };
  if (session) h['Mcp-Session-Id'] = session;
  return h;
}

function rpc(token, session, id, method, params, tag) {
  const body = JSON.stringify({ jsonrpc: '2.0', id, method, params });
  return http.post(MCP, body, { headers: headers(token, session), tags: { tool: tag || method } });
}

export function setup() {
  // OAuth 2.1 client_credentials: what an autonomous agent would do
  const tokenRes = http.post(TOKEN_URL, {
    grant_type: 'client_credentials',
    client_id: CLIENT_ID,
    client_secret: CLIENT_SECRET,
    scope: 'orders:read',
  });
  check(tokenRes, { 'got token': (r) => r.status === 200 });
  const token = tokenRes.json('access_token');

  const init = rpc(token, null, 1, 'initialize', {
    protocolVersion: '2025-06-18',
    capabilities: {},
    clientInfo: { name: 'k6-agent', version: '1.0.0' },
  });
  check(init, { 'initialize ok': (r) => r.status === 200 });
  const session = init.headers['Mcp-Session-Id'];
  http.post(MCP, JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' }),
    { headers: headers(token, session) });
  return { token, session };
}

export default function ({ token, session }) {
  if (Math.random() < 0.7) {
    const orderId = `ORD-${1000 + Math.floor(Math.random() * 60)}`;
    const res = rpc(token, session, 2, 'tools/call',
      { name: 'findOrder', arguments: { orderId } }, 'findOrder');
    check(res, { 'findOrder 200': (r) => r.status === 200, 'has order': (r) => r.body.includes(orderId) });
  } else {
    const customerId = `CUS-${1 + Math.floor(Math.random() * 6)}`;
    const res = rpc(token, session, 3, 'tools/call',
      { name: 'listRecentOrders', arguments: { customerId, limit: 5 } }, 'listRecentOrders');
    check(res, { 'listRecentOrders 200': (r) => r.status === 200 });
  }
  sleep(0.1);
}
