import { BadRequestException } from '@nestjs/common';
import { lookup } from 'node:dns/promises';
import { isIP } from 'node:net';

const BLOCKED_HOST_SUFFIXES = ['.localhost', '.local', '.internal'];

export async function assertSafeWebhookUrl(rawUrl: string): Promise<void> {
  let parsed: URL;

  try {
    parsed = new URL(rawUrl);
  } catch {
    throw invalidWebhookUrl('Webhook URL must be a valid HTTPS URL');
  }

  if (parsed.protocol !== 'https:') {
    throw invalidWebhookUrl('Webhook URL must use HTTPS');
  }

  if (parsed.username || parsed.password) {
    throw invalidWebhookUrl('Webhook URL must not include credentials');
  }

  if (parsed.port && parsed.port !== '443') {
    throw invalidWebhookUrl('Webhook URL must use the default HTTPS port');
  }

  const hostname = parsed.hostname.toLowerCase();
  if (
    hostname === 'localhost' ||
    BLOCKED_HOST_SUFFIXES.some((suffix) => hostname.endsWith(suffix))
  ) {
    throw invalidWebhookUrl('Webhook URL host is not allowed');
  }

  const addresses = isIP(hostname)
    ? [hostname]
    : await resolveWebhookHost(hostname);

  if (addresses.length === 0 || addresses.some(isBlockedAddress)) {
    throw invalidWebhookUrl('Webhook URL resolves to a private or reserved address');
  }
}

async function resolveWebhookHost(hostname: string): Promise<string[]> {
  try {
    const records = await lookup(hostname, { all: true, verbatim: true });
    return records.map((record) => record.address);
  } catch {
    throw invalidWebhookUrl('Webhook URL host could not be resolved');
  }
}

function invalidWebhookUrl(message: string): BadRequestException {
  return new BadRequestException({
    error: {
      code: 'unsafe_webhook_url',
      message,
    },
  });
}

function isBlockedAddress(address: string): boolean {
  const version = isIP(address);

  if (version === 4) {
    return isBlockedIPv4(address);
  }

  if (version === 6) {
    return isBlockedIPv6(address);
  }

  return true;
}

function isBlockedIPv4(address: string): boolean {
  const octets = address.split('.').map((part) => Number(part));
  const [first, second] = octets;

  if (octets.length !== 4 || octets.some((octet) => !Number.isInteger(octet))) {
    return true;
  }

  return (
    first === 0 ||
    first === 10 ||
    first === 127 ||
    first >= 224 ||
    (first === 100 && second >= 64 && second <= 127) ||
    (first === 169 && second === 254) ||
    (first === 172 && second >= 16 && second <= 31) ||
    (first === 192 && second === 168) ||
    (first === 198 && (second === 18 || second === 19))
  );
}

function isBlockedIPv6(address: string): boolean {
  const normalized = address.toLowerCase();

  if (normalized.startsWith('::ffff:')) {
    return isBlockedIPv4(normalized.slice('::ffff:'.length));
  }

  return (
    normalized === '::' ||
    normalized === '::1' ||
    normalized.startsWith('fc') ||
    normalized.startsWith('fd') ||
    normalized.startsWith('fe80') ||
    normalized.startsWith('ff') ||
    normalized.startsWith('2001:db8')
  );
}
