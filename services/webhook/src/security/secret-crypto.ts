import {
  createCipheriv,
  createDecipheriv,
  createHash,
  randomBytes,
} from 'node:crypto';

const PREFIX = 'enc:v1:';

export function encryptSecret(secret: string): string {
  if (isEncryptedSecret(secret)) {
    return secret;
  }

  const iv = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', encryptionKey(), iv);
  const ciphertext = Buffer.concat([
    cipher.update(secret, 'utf8'),
    cipher.final(),
  ]);
  const tag = cipher.getAuthTag();

  return `${PREFIX}${iv.toString('base64url')}.${tag.toString('base64url')}.${ciphertext.toString('base64url')}`;
}

export function decryptSecret(secret: string): string {
  if (!isEncryptedSecret(secret)) {
    return secret;
  }

  const [ivText, tagText, ciphertextText] = secret
    .slice(PREFIX.length)
    .split('.');
  if (!ivText || !tagText || !ciphertextText) {
    throw new Error('Invalid encrypted webhook secret');
  }

  const decipher = createDecipheriv(
    'aes-256-gcm',
    encryptionKey(),
    Buffer.from(ivText, 'base64url'),
  );
  decipher.setAuthTag(Buffer.from(tagText, 'base64url'));

  return Buffer.concat([
    decipher.update(Buffer.from(ciphertextText, 'base64url')),
    decipher.final(),
  ]).toString('utf8');
}

export function isEncryptedSecret(secret: string): boolean {
  return secret.startsWith(PREFIX);
}

function encryptionKey(): Buffer {
  const raw = process.env.WEBHOOK_SECRET_ENCRYPTION_KEY;

  if (!raw) {
    throw new Error('WEBHOOK_SECRET_ENCRYPTION_KEY is required');
  }

  const trimmed = raw.trim();
  if (/^[a-f0-9]{64}$/i.test(trimmed)) {
    return Buffer.from(trimmed, 'hex');
  }

  try {
    const decoded = Buffer.from(trimmed, 'base64');
    if (decoded.length === 32) {
      return decoded;
    }
  } catch {
    // Fall through to deterministic key derivation for non-base64 env values.
  }

  if (isProduction()) {
    throw new Error(
      'WEBHOOK_SECRET_ENCRYPTION_KEY must be a 32-byte key encoded as 64 hex characters or base64 in production',
    );
  }

  return createHash('sha256').update(trimmed).digest();
}

function isProduction(): boolean {
  return (
    process.env.NODE_ENV === 'production' ||
    process.env.ENVIRONMENT === 'production'
  );
}
