// Deterministic stand-in for Finnhub and Twelve Data on one port.
// Control the failure mode out of band: POST /__control {"mode":"ok"}.
import { createServer } from 'node:http';

const port = Number(process.env.FAKE_PROVIDER_PORT ?? 4111);
const host = '127.0.0.1';

const knownSymbols = new Map([
  [
    'AAPL',
    { price: 190.12, previousClose: 188.88, change: 1.24, changePercent: 0.66 },
  ],
  [
    'MSFT',
    { price: 410.5, previousClose: 400, change: 10.5, changePercent: 2.63 },
  ],
  [
    'TSLA',
    { price: 250, previousClose: 250, change: 0, changePercent: 0 },
  ],
]);

let mode = 'ok';
let requestCount = 0;

const quoteTimestamp = () => Math.floor(Date.now() / 1000) - 60;

function sendJson(res, status, body) {
  const payload = JSON.stringify(body);
  res.writeHead(status, {
    'content-type': 'application/json; charset=utf-8',
    'cache-control': 'no-store',
  });
  res.end(payload);
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let raw = '';
    req.on('data', (chunk) => {
      raw += chunk;
      if (raw.length > 64 * 1024) reject(new Error('body too large'));
    });
    req.on('end', () => resolve(raw));
    req.on('error', reject);
  });
}

function finnhubQuote(res, symbol) {
  if (mode === 'finnhub-down' || mode === 'all-down') {
    sendJson(res, 500, { error: 'upstream exploded token=SECRET-XYZ' });
    return;
  }
  const quote = knownSymbols.get(symbol);
  if (!quote) {
    sendJson(res, 200, { c: 0, d: 0, dp: 0, pc: 0, t: 0 });
    return;
  }
  sendJson(res, 200, {
    c: quote.price,
    d: quote.change,
    dp: quote.changePercent,
    pc: quote.previousClose,
    t: quoteTimestamp(),
  });
}

function twelveQuote(res, symbol) {
  if (mode === 'twelve-down' || mode === 'all-down') {
    sendJson(res, 500, { status: 'error', code: 500, message: 'unavailable' });
    return;
  }
  const quote = knownSymbols.get(symbol);
  if (!quote) {
    sendJson(res, 200, {
      code: 400,
      message: 'symbol not found',
      status: 'error',
    });
    return;
  }
  sendJson(res, 200, {
    symbol,
    name: `${symbol} Inc`,
    close: String(quote.price),
    change: String(quote.change),
    percent_change: String(quote.changePercent),
    previous_close: String(quote.previousClose),
    timestamp: String(quoteTimestamp()),
    status: 'ok',
  });
}

function dailySeries(res, symbol, outputsize) {
  if (mode === 'twelve-down' || mode === 'all-down') {
    sendJson(res, 500, { status: 'error', code: 500, message: 'unavailable' });
    return;
  }
  if (!knownSymbols.has(symbol)) {
    sendJson(res, 200, { code: 400, message: 'symbol not found', status: 'error' });
    return;
  }
  const size = Math.min(Math.max(Number(outputsize) || 30, 1), 365);
  const values = [];
  const day = new Date();
  day.setUTCHours(0, 0, 0, 0);
  for (let index = 0; index < size; index += 1) {
    values.push({
      datetime: day.toISOString().slice(0, 10),
      close: String(100 + index * 0.5),
    });
    day.setUTCDate(day.getUTCDate() - 1);
  }
  sendJson(res, 200, {
    meta: { symbol, interval: '1day' },
    values,
    status: 'ok',
  });
}

const server = createServer((req, res) => {
  const url = new URL(req.url ?? '/', `http://${host}:${port}`);
  const symbol = (url.searchParams.get('symbol') ?? '').toUpperCase();

  if (url.pathname === '/__health') {
    sendJson(res, 200, { status: 'ok' });
    return;
  }
  if (url.pathname === '/__stats') {
    sendJson(res, 200, { requests: requestCount });
    return;
  }
  if (url.pathname === '/__control' && req.method === 'POST') {
    readBody(req)
      .then((raw) => {
        const next = JSON.parse(raw || '{}');
        mode = typeof next.mode === 'string' ? next.mode : 'ok';
        requestCount = 0;
        sendJson(res, 200, { mode });
      })
      .catch(() => sendJson(res, 400, { error: 'invalid control body' }));
    return;
  }

  requestCount += 1;
  if (url.pathname === '/api/v1/quote') {
    finnhubQuote(res, symbol);
    return;
  }
  if (url.pathname === '/quote') {
    twelveQuote(res, symbol);
    return;
  }
  if (url.pathname === '/time_series') {
    dailySeries(res, symbol, url.searchParams.get('outputsize'));
    return;
  }
  sendJson(res, 404, { error: 'unknown fixture path' });
});

server.listen(port, host, () => {
  console.log(`fake provider listening on http://${host}:${port}`);
});

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => server.close(() => process.exit(0)));
}
