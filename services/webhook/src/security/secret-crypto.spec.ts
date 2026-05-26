import {
  decryptSecret,
  encryptSecret,
  isEncryptedSecret,
} from './secret-crypto';

describe('webhook secret crypto', () => {
  const originalKey = process.env.WEBHOOK_SECRET_ENCRYPTION_KEY;

  beforeEach(() => {
    process.env.WEBHOOK_SECRET_ENCRYPTION_KEY = 'test-webhook-encryption-key';
  });

  afterEach(() => {
    process.env.WEBHOOK_SECRET_ENCRYPTION_KEY = originalKey;
  });

  it('given a plain webhook secret when encrypted then decrypts back to the original value', () => {
    // Arrange
    const secret = 'whsec_unit_secret';

    // Act
    const encrypted = encryptSecret(secret);
    const decrypted = decryptSecret(encrypted);

    // Assert
    expect(encrypted).not.toBe(secret);
    expect(isEncryptedSecret(encrypted)).toBe(true);
    expect(decrypted).toBe(secret);
  });

  it('given an already encrypted webhook secret when encrypted again then leaves it unchanged', () => {
    // Arrange
    const encrypted = encryptSecret('whsec_once');

    // Act
    const encryptedAgain = encryptSecret(encrypted);

    // Assert
    expect(encryptedAgain).toBe(encrypted);
  });
});
