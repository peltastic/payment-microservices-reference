import { of } from 'rxjs';
import { ProxyService } from './proxy.service';

describe('ProxyService.forward', () => {
  const originalSecret = process.env.INTERNAL_AUTH_SECRET;
  const logger = {
    with: jest.fn(() => ({
      info: jest.fn(),
      error: jest.fn(),
    })),
  };
  const proxyRequestsTotal = { inc: jest.fn() };

  beforeEach(() => {
    jest.clearAllMocks();
    process.env.INTERNAL_AUTH_SECRET = 'proxy-secret';
  });

  afterEach(() => {
    process.env.INTERNAL_AUTH_SECRET = originalSecret;
  });

  it('given a gateway request with query params when forwarded then sends signed merchant headers downstream', async () => {
    // Arrange
    const httpService = {
      request: jest.fn(() => of({ status: 202, data: { accepted: true } })),
    };
    const service = new ProxyService(
      httpService as never,
      logger as never,
      proxyRequestsTotal as never,
    );
    const request = {
      method: 'GET',
      originalUrl: '/v1/payments',
      url: '/v1/payments',
      query: { status: 'completed', tag: ['a', 'b'] },
      body: undefined,
      requestId: 'req_proxy',
      merchantId: 'mrc_proxy',
      merchantScope: 'payments:read',
    };

    // Act
    const result = await service.forward(
      request as never,
      'payment',
      'http://payment:4002/payments/',
    );

    // Assert
    expect(result).toEqual({ status: 202, data: { accepted: true } });
    expect(httpService.request).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'GET',
        url: 'http://payment:4002/payments/?status=completed&tag=a&tag=b',
        headers: expect.objectContaining({
          'x-merchant-id': 'mrc_proxy',
          'x-merchant-scope': 'payments:read',
          'x-request-id': 'req_proxy',
          'x-internal-timestamp': expect.any(String),
          'x-internal-body-sha256': expect.any(String),
          'x-internal-signature': expect.any(String),
        }),
      }),
    );
    expect(proxyRequestsTotal.inc).toHaveBeenCalledWith({
      service: 'payment',
      status: '202',
    });
  });
});
