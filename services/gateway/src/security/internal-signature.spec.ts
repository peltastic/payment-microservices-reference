import { createHmac } from 'node:crypto';
import {
  canonicalInternalRequest,
  hashBody,
  signInternalRequest,
} from './internal-signature';

describe('internal service signatures', () => {
  const originalSecret = process.env.INTERNAL_AUTH_SECRET;

  beforeEach(() => {
    jest.spyOn(Date, 'now').mockReturnValue(1_700_000_000_000);
    process.env.INTERNAL_AUTH_SECRET = 'unit-secret';
  });

  afterEach(() => {
    jest.restoreAllMocks();
    process.env.INTERNAL_AUTH_SECRET = originalSecret;
  });

  it('given a target url with query params when canonicalized then signs only method path query and forwarding identifiers', () => {
    // Arrange
    const input = {
      method: 'post',
      targetUrl: 'http://payment:4002/payments/?page=1',
      timestamp: '1700000000',
      requestId: 'req_unit',
      merchantId: 'mrc_unit',
      bodySha256: hashBody({ amount: 1000 }),
    };

    // Act
    const canonical = canonicalInternalRequest(input);

    // Assert
    expect(canonical).toBe(
      `POST\n/payments/?page=1\n1700000000\nreq_unit\nmrc_unit\n${hashBody({ amount: 1000 })}`,
    );
  });

  it('given a request to sign when secret is configured then returns the expected hmac headers', () => {
    // Arrange
    const body = { amount: 1000 };
    const bodySha256 = hashBody(body);
    const canonical = `POST\n/payments/\n1700000000\nreq_unit\nmrc_unit\n${bodySha256}`;
    const expected = createHmac('sha256', 'unit-secret')
      .update(canonical)
      .digest('hex');

    // Act
    const headers = signInternalRequest({
      method: 'POST',
      targetUrl: 'http://payment:4002/payments/',
      requestId: 'req_unit',
      merchantId: 'mrc_unit',
      body,
    });

    // Assert
    expect(headers).toEqual({
      'x-internal-timestamp': '1700000000',
      'x-internal-body-sha256': bodySha256,
      'x-internal-signature': expected,
    });
  });
});
