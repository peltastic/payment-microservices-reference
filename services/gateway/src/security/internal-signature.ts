import { createHash, createHmac } from 'node:crypto';

const TIMESTAMP_HEADER = 'x-internal-timestamp';
const SIGNATURE_HEADER = 'x-internal-signature';
const BODY_HASH_HEADER = 'x-internal-body-sha256';

function getInternalSecret(): string {
  const secret = process.env.INTERNAL_AUTH_SECRET;

  if (!secret) {
    throw new Error(
      'INTERNAL_AUTH_SECRET is required for internal service calls',
    );
  }

  return secret;
}

function pathWithQuery(targetUrl: string): string {
  const url = new URL(targetUrl);
  return `${url.pathname}${url.search}`;
}

export function canonicalInternalRequest(input: {
  method: string;
  targetUrl: string;
  timestamp: string;
  requestId?: string;
  merchantId?: string;
  bodySha256?: string;
}): string {
  return [
    input.method.toUpperCase(),
    pathWithQuery(input.targetUrl),
    input.timestamp,
    input.requestId ?? '',
    input.merchantId ?? '',
    input.bodySha256 ?? hashBody(undefined),
  ].join('\n');
}

export function signInternalRequest(input: {
  method: string;
  targetUrl: string;
  requestId?: string;
  merchantId?: string;
  body?: unknown;
}): Record<string, string> {
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const bodySha256 = hashBody(input.body);
  const canonical = canonicalInternalRequest({
    ...input,
    timestamp,
    bodySha256,
  });
  const signature = createHmac('sha256', getInternalSecret())
    .update(canonical)
    .digest('hex');
  return {
    [TIMESTAMP_HEADER]: timestamp,
    [BODY_HASH_HEADER]: bodySha256,
    [SIGNATURE_HEADER]: signature,
  };
}

export function hashBody(body: unknown): string {
  if (body === undefined || body === null) {
    return createHash('sha256').update('').digest('hex');
  }

  if (Buffer.isBuffer(body)) {
    return createHash('sha256').update(body).digest('hex');
  }

  if (typeof body === 'string') {
    return createHash('sha256').update(body).digest('hex');
  }

  return createHash('sha256').update(JSON.stringify(body)).digest('hex');
}
